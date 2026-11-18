package serve

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"net/http"
	"time"

	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

//go:embed skills/one-gateway/SKILL.md
var gatewaySkill []byte

func handleGatewaySkillDownload(w http.ResponseWriter, r *http.Request) {
	var bundle bytes.Buffer
	if err := writeGatewaySkillZip(&bundle); err != nil {
		writeError(w, http.StatusInternalServerError, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.skill_archive_failed"), nil)
		return
	}

	const filename = "one-gateway.zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	http.ServeContent(w, r, filename, time.Time{}, bytes.NewReader(bundle.Bytes()))
}

func writeGatewaySkillZip(bundle *bytes.Buffer) error {
	archive := zip.NewWriter(bundle)
	file, err := archive.Create("one-gateway/SKILL.md")
	if err != nil {
		return err
	}
	if _, err := file.Write(gatewaySkill); err != nil {
		return err
	}
	return archive.Close()
}
