//go:build !windows

package process

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestTerminateTree(t *testing.T) {
	for _, mode := range []string{"graceful", "stubborn", "spawn-on-stop", "parent-exits-first"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			cmd := exec.Command(os.Args[0], "-test.run=^TestTerminateTreeHelper$")
			cmd.Env = append(os.Environ(), "ONE_STOP_HELPER=parent", "ONE_STOP_CHILD="+mode, "ONE_STOP_ROOT="+root)
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if cmd.ProcessState == nil {
					_ = StopTree(cmd.Process)
					_ = cmd.Wait()
				}
			})
			deadline := time.Now().Add(5 * time.Second)
			var pid int
			for {
				data, _ := os.ReadFile(filepath.Join(root, "ready"))
				pid, _ = strconv.Atoi(string(data))
				if pid > 0 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("child did not start")
				}
				time.Sleep(10 * time.Millisecond)
			}
			started := time.Now()
			if err := TerminateTree(cmd.Process, 250*time.Millisecond); err != nil {
				t.Fatal(err)
			}
			if err := cmd.Wait(); err == nil {
				t.Fatal("parent was not stopped")
			}
			if elapsed := time.Since(started); elapsed < 250*time.Millisecond || elapsed > 2*time.Second {
				t.Fatalf("unexpected shutdown duration: %v", elapsed)
			}
			if mode == "graceful" {
				if _, err := os.Stat(filepath.Join(root, "cleaned")); err != nil {
					t.Fatal("child did not receive time to clean up")
				}
			}
			if mode == "spawn-on-stop" || mode == "parent-exits-first" {
				data, err := os.ReadFile(filepath.Join(root, "late", "ready"))
				if err != nil {
					t.Fatal("parent did not spawn during shutdown:", err)
				}
				pid, err = strconv.Atoi(string(data))
				if err != nil {
					t.Fatal(err)
				}
			}
			// An orphan may briefly be a zombie until the OS reaps it.
			out, _ := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "stat=").Output()
			if state := strings.TrimSpace(string(out)); state != "" && !strings.HasPrefix(state, "Z") {
				_ = syscall.Kill(pid, syscall.SIGKILL)
				t.Fatalf("descendant in its own process group survived: %s", state)
			}
		})
	}
}

func TestTerminateTreeHelper(t *testing.T) {
	mode := os.Getenv("ONE_STOP_HELPER")
	if mode == "" {
		return
	}
	root := os.Getenv("ONE_STOP_ROOT")
	if mode == "parent" {
		childMode := os.Getenv("ONE_STOP_CHILD")
		if childMode == "spawn-on-stop" {
			stopped := make(chan os.Signal, 1)
			signal.Notify(stopped, syscall.SIGTERM)
			if err := os.WriteFile(filepath.Join(root, "ready"), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
				os.Exit(1)
			}
			<-stopped
			root = filepath.Join(root, "late")
			if err := os.Mkdir(root, 0700); err != nil {
				os.Exit(1)
			}
			childMode = "stubborn"
		} else if childMode == "parent-exits-first" {
			// Model a shell wrapper that exits before its child's cleanup.
			signal.Reset(syscall.SIGTERM)
		} else {
			signal.Ignore(syscall.SIGTERM)
		}
		child := exec.Command(os.Args[0], "-test.run=^TestTerminateTreeHelper$")
		child.Env = append(os.Environ(), "ONE_STOP_HELPER="+childMode, "ONE_STOP_ROOT="+root)
		child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := child.Start(); err != nil {
			os.Exit(1)
		}
		_ = child.Wait()
		for {
			time.Sleep(time.Hour)
		}
	}
	originalParent := os.Getppid()
	stopped := make(chan os.Signal, 1)
	if mode == "graceful" || mode == "parent-exits-first" {
		signal.Notify(stopped, syscall.SIGTERM)
	} else {
		signal.Ignore(syscall.SIGTERM)
	}
	if err := os.WriteFile(filepath.Join(root, "ready"), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		os.Exit(1)
	}
	if mode == "parent-exits-first" {
		<-stopped
		deadline := time.Now().Add(time.Second)
		for os.Getppid() == originalParent {
			if time.Now().After(deadline) {
				os.Exit(1)
			}
			time.Sleep(time.Millisecond)
		}
		late := filepath.Join(root, "late")
		if err := os.Mkdir(late, 0700); err != nil {
			os.Exit(1)
		}
		child := exec.Command(os.Args[0], "-test.run=^TestTerminateTreeHelper$")
		child.Env = append(os.Environ(), "ONE_STOP_HELPER=stubborn", "ONE_STOP_ROOT="+late)
		child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := child.Start(); err != nil {
			os.Exit(1)
		}
		_ = child.Wait()
		os.Exit(0)
	}
	if mode == "graceful" {
		<-stopped
		time.Sleep(50 * time.Millisecond)
		if err := os.WriteFile(filepath.Join(root, "cleaned"), []byte("done"), 0600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	for {
		time.Sleep(time.Hour)
	}
}

func TestRefreshTreeRetainsReparentedChildren(t *testing.T) {
	worker := treeProcess{pid: 2, parent: 1, started: "worker"}
	cleanup := treeProcess{pid: 3, parent: 2, started: "cleanup"}
	tracked := refreshTree([]treeProcess{worker}, map[int]treeProcess{2: worker, 3: cleanup})
	// The worker exits and its PID is reused by an unrelated process. Its
	// previously discovered cleanup child is reparented and spawns another child.
	cleanup.parent = 1
	leaf := treeProcess{pid: 4, parent: 3, started: "leaf"}
	tracked = refreshTree(tracked, map[int]treeProcess{
		2: {pid: 2, parent: 1, started: "unrelated"},
		3: cleanup, 4: leaf,
		5: {pid: 5, parent: 2, started: "unrelated-child"},
	})
	if len(tracked) != 2 {
		t.Fatalf("unexpected ownership set: %+v", tracked)
	}
	seen := map[int]bool{}
	for _, p := range tracked {
		seen[p.pid] = true
	}
	if !seen[3] || !seen[4] {
		t.Fatalf("lost reparented cleanup subtree: %+v", tracked)
	}
}
