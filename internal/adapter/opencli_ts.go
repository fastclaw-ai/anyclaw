package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/fastclaw-ai/anyclaw/internal/pkg"
)

// OpenCLITSAdapter executes opencli TypeScript packages via a Node.js shim
// that provides browser automation through the daemon.
type OpenCLITSAdapter struct{}

func (a *OpenCLITSAdapter) Execute(ctx context.Context, cmd *pkg.Command, params map[string]any, packageDir string) (*Result, error) {
	if cmd.Script == nil || cmd.Script.Code == "" {
		return nil, fmt.Errorf("command %q missing script code", cmd.Name)
	}

	// Check node is available
	nodePath, err := exec.LookPath("node")
	if err != nil {
		return nil, fmt.Errorf("node not found — install Node.js to run opencli-ts packages")
	}

	// Ensure daemon is running for browser automation
	EnsureDaemon()

	// Strip TypeScript-specific syntax from the code
	code := stripTypeScript(cmd.Script.Code)

	// Build the full script with shim + user code
	script := buildTSShim(code)

	// Serialize args as JSON env var
	argsJSON, _ := json.Marshal(params)

	c := exec.CommandContext(ctx, nodePath, "--input-type=module")
	c.Dir = packageDir
	c.Stdin = strings.NewReader(script)
	c.Env = append(os.Environ(), "ANYCLAW_ARGS="+string(argsJSON))

	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr

	if err := c.Run(); err != nil {
		return nil, fmt.Errorf("opencli-ts: %w\nstderr: %s", err, stderr.String())
	}

	content := strings.TrimSpace(stdout.String())

	var data map[string]any
	if err := json.Unmarshal([]byte(content), &data); err == nil {
		return &Result{Content: content, Data: data}, nil
	}

	return &Result{Content: content}, nil
}

// stripTypeScript removes TypeScript-specific syntax to produce valid JavaScript.
func stripTypeScript(code string) string {
	// Remove import statements
	reImport := regexp.MustCompile(`(?m)^import\s+.*?;\s*$`)
	code = reImport.ReplaceAllString(code, "")

	// Remove type annotations on parameters: (param: Type) -> (param)
	reParamType := regexp.MustCompile(`(\w)\s*:\s*[A-Z]\w*(?:<[^>]*>)?(\s*[,)])`)
	code = reParamType.ReplaceAllString(code, "$1$2")

	// Remove return type annotations: ): Type => -> ) => and ): Type { -> ) {
	reReturnType := regexp.MustCompile(`\)\s*:\s*[A-Z]\w*(?:<[^>]*>)?\s*([{=])`)
	code = reReturnType.ReplaceAllString(code, ") $1")

	// Remove "as Type" casts
	reAsCast := regexp.MustCompile(`\s+as\s+[A-Z]\w*(?:<[^>]*>)?`)
	code = reAsCast.ReplaceAllString(code, "")

	// Remove standalone generic type parameters on function calls: func<Type>(
	reGeneric := regexp.MustCompile(`(<[A-Z]\w*(?:\[\])?(?:\s*,\s*[A-Z]\w*(?:\[\])?)*)>(\()`)
	code = reGeneric.ReplaceAllString(code, "$2")

	return code
}

// buildTSShim wraps user code with the Node.js compatibility shim.
func buildTSShim(userCode string) string {
	return `
const DAEMON_PORT = parseInt(process.env.ANYCLAW_DAEMON_PORT || "19825");
const args = JSON.parse(process.env.ANYCLAW_ARGS || "{}");

const Strategy = { COOKIE: "cookie", INTERCEPT: "intercept", PINIA: "pinia", DOM: "dom" };

async function daemonCommand(action, extra) {
  const body = { id: "ts_" + Date.now(), action, ...extra };
  const resp = await fetch("http://127.0.0.1:" + DAEMON_PORT + "/command", {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-OpenCLI": "1" },
    body: JSON.stringify(body),
  });
  const data = await resp.json();
  if (!data.ok) throw new Error(data.error || "daemon error");
  return data;
}

const page = {
  async goto(url) {
    await daemonCommand("navigate", { url });
    await new Promise(r => setTimeout(r, 2000));
  },
  async evaluate(script) {
    const r = await daemonCommand("exec", { code: String(script) });
    return r.data ?? r;
  },
  async wait(n) {
    await new Promise(r => setTimeout(r, (n || 1) * 1000));
  },
};

let _result;
function cli(def) {
  _result = def.func(page, args)
    .then(r => console.log(JSON.stringify(r, null, 2)))
    .catch(e => { console.error(e); process.exit(1); });
}

` + userCode + `

await _result;
`
}
