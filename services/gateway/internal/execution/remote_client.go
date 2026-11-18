package execution

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issueworkflow"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/userdirs"
	"github.com/torchstellar-team/one-cli/services/gateway/internal/gateway"
)

type workerConfig struct {
	URL   string `json:"url"`
	ID    string `json:"id"`
	Name  string `json:"name"`
	Token string `json:"token"`
}

type remoteLink struct {
	config workerConfig
	base   *url.URL
	http   *http.Client
}

func newRemote(config workerConfig) (remoteLink, error) {
	base, err := url.Parse(config.URL)
	if err != nil || base.Hostname() == "" || base.User != nil || base.RawQuery != "" || base.ForceQuery ||
		base.Fragment != "" || (base.Scheme != "http" && base.Scheme != "https") {
		return remoteLink{}, i18n.Errorf("gateway.worker_address")
	}
	if !workerIDPattern.MatchString(config.ID) || !api.ValidToken(config.Token) || !validWorkerName(config.Name) {
		return remoteLink{}, i18n.Errorf("gateway.worker_config")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return remoteLink{config: config, base: base, http: &http.Client{
		Transport:     transport,
		Timeout:       5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func (l remoteLink) send(ctx context.Context, endpoint *url.URL, input, out any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+l.config.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(mirrorCapabilityHeader, "1")
	resp, err := l.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		var problem struct{ Reason string }
		_ = decoder.Decode(&problem)
		return &api.ResponseError{Code: resp.StatusCode, Reason: problem.Reason}
	}
	return decoder.Decode(out)
}

func (l remoteLink) endpoint(op string) *url.URL {
	return l.base.JoinPath("worker", "v1", "workers", l.config.ID, op)
}

func (l remoteLink) heartbeat(ctx context.Context, session string, inventory api.ExecutionWorker) error {
	return l.send(ctx, l.endpoint("heartbeat"), heartbeatRequest{Session: session, Inventory: inventory}, new(struct{}))
}

func (l remoteLink) claim(ctx context.Context, session string) (*assignment, error) {
	var item *assignment
	if err := l.send(ctx, l.endpoint("claim"), claimRequest{Session: session}, &item); err != nil {
		return nil, err
	}
	if item != nil && (!api.ValidExecutionID(item.ID) || item.WorkerID != l.config.ID ||
		item.Session != session || !validSession(item.Attempt) || !item.ExecutionSubmission.Valid()) {
		return nil, i18n.Errorf("gateway.worker_assignment")
	}
	return item, nil
}

func (l remoteLink) complete(ctx context.Context, input report) error {
	return l.sendReport(ctx, "complete", input)
}

func (l remoteLink) upload(ctx context.Context, input report) error {
	return l.sendReport(ctx, "logs", input)
}

func (l remoteLink) sendReport(ctx context.Context, operation string, input report) error {
	endpoint := l.endpoint(operation)
	err := l.send(ctx, endpoint, input, new(struct{}))
	if input.Result.EffectiveCodex != (api.CodexOptions{}) && rejectsWorkerReport(err) {
		input.Result.EffectiveCodex = api.CodexOptions{}
		return l.send(ctx, endpoint, input, new(struct{}))
	}
	return err
}

func rejectsWorkerReport(err error) bool {
	var response *api.ResponseError
	return errors.As(err, &response) && response.Code == http.StatusBadRequest && response.Reason == "invalid_worker_report"
}

func (l remoteLink) exchangeQuestion(ctx context.Context, input questionReport) (api.ExecutionQuestion, error) {
	var question api.ExecutionQuestion
	err := l.send(ctx, l.endpoint("question"), input, &question)
	return question, err
}

func readWorkerConfig(path string) (workerConfig, error) {
	var cfg workerConfig
	info, err := os.Stat(path)
	if err != nil {
		return cfg, err
	}
	if !info.Mode().IsRegular() || info.Size() > 8<<10 || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
		return cfg, i18n.Errorf("gateway.worker_config")
	}
	err = readJSON(path, &cfg)
	return cfg, err
}

func Join(ctx context.Context, endpoint, name, code string, replace bool) (string, error) {
	dir, err := userdirs.Config()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "gateway-worker.json")
	lock := flock.New(path + ".lock")
	locked, err := lock.TryLock()
	if err != nil {
		return "", err
	}
	if !locked {
		return "", i18n.Errorf("gateway.worker_config")
	}
	defer lock.Close()

	proposed := path
	if replace {
		if err := ValidateWorkerHTTPS(endpoint); err != nil {
			return "", err
		}
		proposed = path + ".pending"
	}
	cfg, err := readWorkerConfig(proposed)
	if errors.Is(err, os.ErrNotExist) {
		cfg = workerConfig{URL: endpoint, Name: name, ID: "w-" + strings.ToLower(rand.Text()), Token: rand.Text() + rand.Text()}
	} else if err != nil {
		return "", err
	} else if cfg.URL != endpoint || cfg.Name != name {
		return "", i18n.Errorf("gateway.worker_already_configured", cfg.Name, cfg.URL)
	}
	client, err := newRemote(cfg)
	if err != nil {
		return "", err
	}
	defer client.http.CloseIdleConnections()
	// Persist the proposed identity so a lost join response can be retried.
	if err := writeJSON(proposed, cfg); err != nil {
		return "", err
	}
	if err := client.send(ctx, client.base.JoinPath("worker", "v1", "join"), joinRequest{ID: cfg.ID, Name: cfg.Name, Code: code}, new(struct{})); err != nil {
		return "", i18n.Errorf("gateway.worker_join_failed", err)
	}
	if replace {
		if err := writeJSON(path, cfg); err != nil {
			return "", err
		}
		_ = os.Remove(proposed)
	}
	return cfg.ID, nil
}

func RunRemote(ctx context.Context, one, workflowConfig string) error {
	paths, err := userdirs.Resolve()
	if err != nil {
		return err
	}
	cfg, err := readWorkerConfig(filepath.Join(paths.Config, "gateway-worker.json"))
	if err != nil {
		return i18n.Errorf("gateway.worker_config_read", err)
	}
	client, err := newRemote(cfg)
	if err != nil {
		return err
	}
	defer client.http.CloseIdleConnections()
	root := filepath.Join(paths.State, "gateway-worker", cfg.ID)
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	lock := flock.New(filepath.Join(root, "lock"))
	locked, err := lock.TryLock()
	if err != nil {
		return err
	}
	if !locked {
		return i18n.Errorf("gateway.worker_running", cfg.Name)
	}
	defer lock.Close()
	runner, err := NewRunner(root, one, nil)
	if err != nil {
		return err
	}
	secure := client.base.Scheme == "https"
	if secure && workflowConfig == "" {
		workflowConfig = filepath.Join(paths.Config, "gateway-worker-workflow-"+cfg.ID+".json")
		if _, err := os.Stat(workflowConfig); errors.Is(err, os.ErrNotExist) {
			initial := issueworkflow.Config{StateDir: filepath.Join(root, "workflow"), Projects: map[string]issueworkflow.Project{}}
			if err := writeJSON(workflowConfig, initial); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	if workflowConfig == "" {
		return runWorker(ctx, client, runner)
	}
	workflow, err := issueworkflow.LoadConfig(workflowConfig)
	if err != nil {
		return err
	}
	runner.readRoots = workflow.CodexReadRoots
	var supervisor gateway.Supervisor
	runner.workflow = supervisor.Call
	ctx, cancel := context.WithCancel(ctx)
	var background sync.WaitGroup
	background.Go(func() { supervisor.Run(ctx, one, workflowConfig) })
	background.Go(func() { client.syncTaskSnapshots(ctx, supervisor.Call) })
	if secure {
		setup := newSetupService(ctx, runner)
		background.Go(func() { client.runSetup(ctx, setup.handle) })
	}
	defer func() {
		cancel()
		background.Wait()
	}()
	return runWorker(ctx, client, runner)
}
