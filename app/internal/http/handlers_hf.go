package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"app/internal/hf"
	ocdscripts "deploy-scripts"
)

// HandleHFParseEmail accepts multipart/form-data with an .eml file under field name "file"
func HandleHFParseEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(20 << 20); err != nil { // 20MB limit
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Failed to parse multipart form: %v", err),
		})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Missing file upload",
		})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	var result *hf.ParsedEmail
	if ext == ".eml" {
		// Parse directly
		parsed, perr := hf.ParseEML(header.Filename, file, hf.DefaultParseOptions())
		if perr != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": fmt.Sprintf("Failed to parse email: %v", perr),
			})
			return
		}
		result = parsed
	} else if ext == ".msg" {
		if runtime.GOOS != "windows" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "MSG parsing is currently supported on Windows only. Please Save As .eml or use Browse to select an .eml file.",
			})
			return
		}
		// Save upload to temp and convert to .eml using Outlook COM via PowerShell
		tempMsg, err := os.CreateTemp("", "hf_upload_*.msg")
		if err != nil {
			http.Error(w, "Failed to create temp file", http.StatusInternalServerError)
			return
		}
		defer os.Remove(tempMsg.Name())
		if _, err := io.Copy(tempMsg, file); err != nil {
			http.Error(w, "Failed to save upload", http.StatusInternalServerError)
			return
		}
		_ = tempMsg.Close()

		emlPath, err := convertMSGToEML(tempMsg.Name())
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": fmt.Sprintf("Failed to convert MSG to EML: %v", err),
			})
			return
		}
		defer os.Remove(emlPath)
		emlFile, err := os.Open(emlPath)
		if err != nil {
			http.Error(w, "Failed to read converted EML", http.StatusInternalServerError)
			return
		}
		defer emlFile.Close()
		parsed, perr := hf.ParseEML(filepath.Base(emlPath), emlFile, hf.DefaultParseOptions())
		if perr != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": fmt.Sprintf("Failed to parse converted EML: %v", perr),
			})
			return
		}
		result = parsed
	} else {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Unsupported file type. Please upload an .eml or .msg file.",
		})
		return
	}
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Failed to parse email: %v", err),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"message":  "Email parsed successfully",
		"filename": result.Filename,
		"subject":  result.Subject,
		"from":     result.From,
		"to":       result.To,
		"versions": result.Versions,
		"raw":      result.RawMappings,
		"html":     result.HTMLSnippet,
	})
}

// HandleHFUpdatePOM updates pom.xml versions based on provided mapping (stub for now)
func HandleHFUpdatePOM(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		RepoURL   string            `json:"repo_url"`
		Branch    string            `json:"branch"`
		Versions  map[string]string `json:"versions"`
		DryRun    bool              `json:"dry_run"`
		PomPath   string            `json:"pom_path"`   // optional, default root pom.xml
		WorkDir   string            `json:"work_dir"`   // optional temp dir override
		CommitMsg string            `json:"commit_msg"` // optional
		Username  string            `json:"username"`   // optional for https auth
		Token     string            `json:"token"`
		Debug     bool              `json:"debug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Invalid request format",
		})
		return
	}

	if req.RepoURL == "" || req.Branch == "" || len(req.Versions) == 0 {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "repo_url, branch and versions are required",
		})
		return
	}

	// Create temp workdir
	workDir := req.WorkDir
	if workDir == "" {
		var err error
		workDir, err = os.MkdirTemp("", "hf_repo_*")
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": fmt.Sprintf("Failed to create temp dir: %v", err),
			})
			return
		}
		keepWorkDir := req.Debug || r.URL.Query().Get("debug") == "1" || r.Header.Get("X-Debug") == "1" || os.Getenv("OCD_DEBUG") == "1"
		if !keepWorkDir {
			defer os.RemoveAll(workDir)
		}
	}

	// Prepare values
	repoURL := req.RepoURL

	// git clone and checkout via embedded script (handles proxy)
	if out, err := runShell(workDir, "git-clone.sh", "--repo", repoURL, "--branch", req.Branch, "--dir", filepath.Join(workDir, "repo"), "--username", req.Username, "--token", req.Token); err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("git clone failed: %v | %s", err, out),
		})
		return
	}

	repoPath := filepath.Join(workDir, "repo")
	pomPath := req.PomPath
	if pomPath == "" {
		pomPath = filepath.Join(repoPath, "pom.xml")
	} else {
		pomPath = filepath.Join(repoPath, pomPath)
	}

	// read current pom
	pomBytes, err := os.ReadFile(pomPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Failed to read pom.xml: %v", err),
		})
		return
	}

	// build diff
	keys := make([]string, 0, len(req.Versions))
	for k := range req.Versions {
		keys = append(keys, k)
	}
	current := hf.ExtractVersions(pomBytes, keys)
	diff := hf.BuildDiff(current, req.Versions)

	if req.DryRun {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Dry-run diff generated",
			"diff":    diff,
			"pretty":  hf.PrettyDiffText(diff),
		})
		return
	}

	// Apply changes
	updated, err := hf.UpdatePOMVersions(pomBytes, req.Versions)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Failed to update pom: %v", err),
		})
		return
	}
	if err := os.WriteFile(pomPath, updated, 0644); err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Failed to write pom.xml: %v", err),
		})
		return
	}

	// git commit and push via embedded script (handles proxy)
	// Build commit message and include HF token from subject if available (X-HF-Subject header)
	msg := req.CommitMsg
	if msg == "" {
		dop := ""
		if raw := r.Header.Get("X-HF-Subject"); raw != "" {
			// Decode MIME encoded words if present
			if decoded, err := new(mime.WordDecoder).DecodeHeader(raw); err == nil && decoded != "" {
				raw = decoded
			}
			// Remove known prefixes not needed in commit title
			raw = regexp.MustCompile(`(?i)^\s*Releases\s*»\s*ReleaseForHF\s*-?\s*`).ReplaceAllString(raw, "")
			// Try to extract version like 10.4.826-hf2503.41 from subject
			if m := regexp.MustCompile(`\b\d+\.\d+\.\d+-hf\d{4}\.\d+\b`).FindString(raw); m != "" {
				dop = m
			} else if m := regexp.MustCompile(`\bhf\d{4}\b`).FindString(strings.ToLower(raw)); m != "" {
				dop = strings.ToUpper(m)
			} else {
				// Fallback: use full subject (no truncation)
				dop = strings.TrimSpace(raw)
			}
		}
		if dop != "" {
			msg = fmt.Sprintf("OCD: HF Adoption - update versions (%s)", dop)
		} else {
			msg = "OCD: HF Adoption - update versions"
		}
	}
	if out, err := runShell(workDir, "git-commit-push.sh", "--repo-dir", repoPath, "--branch", req.Branch, "--message", msg, "--username", req.Username, "--token", req.Token); err != nil {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("git commit/push failed: %v | %s", err, out),
		}
		if req.Debug || r.URL.Query().Get("debug") == "1" {
			resp["work_dir"] = workDir
			resp["repo_dir"] = repoPath
			resp["output"] = out
		}
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "POM updated and pushed successfully",
		"diff":    diff,
	})
}

// runShell executes an embedded script similarly to other modules (ensures proxy on)
func runShell(workDir string, scriptName string, args ...string) (string, error) {
	// Create temp script file from embedded scripts
	tempScriptFile, err := os.CreateTemp("", "HF_*.sh")
	if err != nil {
		return "", fmt.Errorf("failed to create temp script: %v", err)
	}
	defer os.Remove(tempScriptFile.Name())

	// Read embedded script via deploy-scripts module
	content, err := ocdscripts.ReadScript(scriptName)
	if err != nil {
		return "", fmt.Errorf("failed to read embedded script %s: %v", scriptName, err)
	}

	// Normalize line endings
	scriptContent := strings.ReplaceAll(string(content), "\r\n", "\n")
	scriptContent = strings.ReplaceAll(scriptContent, "\r", "\n")
	if err := os.WriteFile(tempScriptFile.Name(), []byte(scriptContent), 0755); err != nil {
		return "", fmt.Errorf("failed to write temp script: %v", err)
	}

	// Build execution command per OS, using bash with login shell for proxy function availability
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		if _, err := exec.LookPath("wsl"); err == nil {
			wslPath := convertToWSLPath(tempScriptFile.Name())
			// Convert known path args for WSL
			wslArgs := convertArgsForWSL(args)
			cmd = exec.Command("wsl", "bash", "-l", "-c", fmt.Sprintf("bash %s %s", wslPath, shellJoin(wslArgs)))
		} else {
			// Try git-bash or bash if present
			cmd = exec.Command("bash", "-l", "-c", fmt.Sprintf("bash %s %s", tempScriptFile.Name(), shellJoin(args)))
		}
	case "linux", "darwin":
		cmd = exec.Command("bash", "-l", "-c", fmt.Sprintf("bash %s %s", tempScriptFile.Name(), shellJoin(args)))
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func shellJoin(args []string) string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if strings.ContainsAny(a, " \t\n'\"") {
			out = append(out, strconv.Quote(a))
		} else {
			out = append(out, a)
		}
	}
	return strings.Join(out, " ")
}

func convertArgsForWSL(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		// Convert Windows paths like C:\Users\... to /mnt/c/Users/...
		if len(a) >= 3 && ((a[1] == ':' && (a[2] == '\\' || a[2] == '/')) || (len(a) > 2 && a[1] == ':')) {
			out = append(out, convertToWSLPath(a))
		} else {
			out = append(out, a)
		}
	}
	return out
}

// convertMSGToEML converts a .msg to .eml on Windows via Outlook COM (PowerShell)
func convertMSGToEML(msgPath string) (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("unsupported OS for MSG conversion")
	}
	emlPath := strings.TrimSuffix(msgPath, filepath.Ext(msgPath)) + ".eml"
	ps := fmt.Sprintf(`$ErrorActionPreference='Stop';$msg='%s';$eml='%s';$ol=New-Object -ComObject Outlook.Application;$m=$ol.Session.OpenSharedItem($msg);$m.SaveAs($eml,5)`, msgPath, emlPath)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("powershell conversion failed: %v | %s", err, string(out))
	}
	if _, err := os.Stat(emlPath); err != nil {
		return "", fmt.Errorf("eml not created: %v", err)
	}
	return emlPath, nil
}

// (runGit*, respondGitError, tail) removed after switching to shell-based flow
