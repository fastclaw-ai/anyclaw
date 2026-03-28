package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fastclaw-ai/anyclaw/internal/pkg"
)

// PipelineAdapter executes YAML pipelines.
type PipelineAdapter struct{}

func (a *PipelineAdapter) Execute(ctx context.Context, cmd *pkg.Command, params map[string]any, packageDir string) (*Result, error) {
	if cmd.Pipeline == nil || len(cmd.Pipeline) == 0 {
		return nil, fmt.Errorf("command %q has no pipeline", cmd.Name)
	}

	// Current data flowing through the pipeline
	var data any

	// Browser-dependent pipelines: auto-start daemon, use browser extension
	if pipelineNeedsBrowser(cmd.Pipeline) {
		EnsureDaemon()

		var connected bool
		for i := 0; i < 10; i++ {
			if connected, _ = BridgeStatus(); connected {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}

		if connected {
			return a.executeWithBridge(ctx, cmd, params)
		}
		return nil, fmt.Errorf("this command requires the AnyClaw browser extension.\nInstall it:\n  1. Open chrome://extensions\n  2. Enable Developer Mode\n  3. Load unpacked → select the 'extension' directory\n  See: https://github.com/fastclaw-ai/anyclaw")
	}

	for _, step := range cmd.Pipeline {
		var err error
		for op, arg := range step {
			switch op {
			case "fetch":
				data, err = pipelineFetch(ctx, arg, params, data)
			case "select":
				data, err = pipelineSelect(data, renderTemplate(fmt.Sprintf("%v", arg), params, nil, 0))
			case "map":
				data, err = pipelineMap(data, arg, params)
			case "filter":
				data, err = pipelineFilter(data, fmt.Sprintf("%v", arg), params)
			case "limit":
				data, err = pipelineLimit(data, renderTemplate(fmt.Sprintf("%v", arg), params, nil, 0))
			case "navigate":
				continue
			case "evaluate":
				return nil, fmt.Errorf("this command requires the AnyClaw browser extension")
			default:
				return nil, fmt.Errorf("unknown pipeline step: %s", op)
			}
			if err != nil {
				return nil, fmt.Errorf("pipeline step %q: %w", op, err)
			}
		}
	}

	// Format output
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, err
	}

	var dataMap map[string]any
	json.Unmarshal(out, &dataMap)

	return &Result{Content: string(out), Data: dataMap}, nil
}

func pipelineFetch(ctx context.Context, arg any, params map[string]any, data any) (any, error) {
	switch v := arg.(type) {
	case string:
		url := renderTemplate(v, params, nil, 0)
		// If URL contains item references and data is an array, fetch per item
		if strings.Contains(v, "${{ item") {
			return fetchPerItem(ctx, v, params, data)
		}
		return httpGet(ctx, url)
	case map[string]any:
		url, _ := v["url"].(string)

		// If URL contains item references and data is an array, fetch per item
		if strings.Contains(url, "${{ item") {
			return fetchPerItem(ctx, url, params, data)
		}

		url = renderTemplate(url, params, nil, 0)

		// Check for params
		if p, ok := v["params"]; ok {
			paramsMap, _ := p.(map[string]any)
			rendered := make(map[string]any)
			for k, val := range paramsMap {
				rendered[k] = renderTemplate(fmt.Sprintf("%v", val), params, nil, 0)
			}

			method, _ := v["method"].(string)
			if strings.ToUpper(method) == "POST" {
				return httpPost(ctx, url, rendered)
			}
			return httpGetWithParams(ctx, url, rendered)
		}

		return httpGet(ctx, url)
	}
	return nil, fmt.Errorf("invalid fetch argument: %T", arg)
}

// fetchPerItem fetches a URL for each item in the data array.
func fetchPerItem(ctx context.Context, urlTmpl string, params map[string]any, data any) (any, error) {
	items, ok := toSlice(data)
	if !ok {
		return nil, fmt.Errorf("fetch per-item: data is not an array")
	}

	var results []any
	for i, item := range items {
		itemMap, _ := item.(map[string]any)
		url := renderTemplate(urlTmpl, params, itemMap, i)
		result, err := httpGet(ctx, url)
		if err != nil {
			continue // skip failed items
		}
		results = append(results, result)
	}
	return results, nil
}

func pipelineSelect(data any, path string) (any, error) {
	parts := strings.Split(path, ".")
	current := data
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("cannot select %q: not an object", part)
		}
		current, ok = m[part]
		if !ok {
			return nil, fmt.Errorf("field %q not found", part)
		}
	}
	return current, nil
}

func pipelineMap(data any, mapping any, params map[string]any) (any, error) {
	items, ok := toSlice(data)
	if !ok {
		return nil, fmt.Errorf("map: data is not an array")
	}

	fieldMap, ok := mapping.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("map: mapping is not an object")
	}

	var result []map[string]any
	for i, item := range items {
		itemMap, _ := item.(map[string]any)
		// For primitive items (numbers, strings), make "item" reference the value itself
		if itemMap == nil {
			itemMap = map[string]any{"__value__": item}
		}
		row := make(map[string]any)
		for key, tmpl := range fieldMap {
			tmplStr := fmt.Sprintf("%v", tmpl)
			// Handle bare ${{ item }} (the whole item, not a field)
			if strings.TrimSpace(tmplStr) == "${{ item }}" {
				row[key] = item
			} else {
				row[key] = renderTemplate(tmplStr, params, itemMap, i)
			}
		}
		result = append(result, row)
	}
	return result, nil
}

func pipelineFilter(data any, expr string, params map[string]any) (any, error) {
	items, ok := toSlice(data)
	if !ok {
		return nil, fmt.Errorf("filter: data is not an array")
	}

	var result []any
	for _, item := range items {
		itemMap, _ := item.(map[string]any)
		if itemMap == nil {
			continue
		}
		if evalFilterExpr(expr, itemMap) {
			result = append(result, item)
		}
	}
	return result, nil
}

// evalFilterExpr evaluates simple filter expressions like:
//
//	"item.title" (truthy check)
//	"item.title && !item.deleted" (AND with negation)
func evalFilterExpr(expr string, item map[string]any) bool {
	// Split on &&
	parts := strings.Split(expr, "&&")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		negate := false
		if strings.HasPrefix(part, "!") {
			negate = true
			part = strings.TrimPrefix(part, "!")
			part = strings.TrimSpace(part)
		}

		// Resolve field value
		var val any
		if strings.HasPrefix(part, "item.") {
			field := strings.TrimPrefix(part, "item.")
			val = resolveNestedValue(item, field)
		} else {
			val = part
		}

		truthy := isTruthy(val)
		if negate {
			truthy = !truthy
		}
		if !truthy {
			return false
		}
	}
	return true
}

// evalSimpleTernary handles expressions like:
// args.sort === 'date' ? 'search_by_date' : 'search'
func evalSimpleTernary(expr string, args map[string]any) string {
	qIdx := strings.Index(expr, "?")
	cIdx := strings.LastIndex(expr, ":")
	if qIdx < 0 || cIdx < 0 || cIdx <= qIdx {
		return ""
	}

	condition := strings.TrimSpace(expr[:qIdx])
	trueVal := strings.Trim(strings.TrimSpace(expr[qIdx+1:cIdx]), "'\"")
	falseVal := strings.Trim(strings.TrimSpace(expr[cIdx+1:]), "'\"")

	// Parse condition: args.X === 'value' or args.X == 'value'
	var field, op, expected string
	for _, sep := range []string{"===", "==", "!=="} {
		if parts := strings.SplitN(condition, sep, 2); len(parts) == 2 {
			field = strings.TrimSpace(parts[0])
			op = sep
			expected = strings.Trim(strings.TrimSpace(parts[1]), "'\"")
			break
		}
	}

	if field == "" {
		return falseVal
	}

	// Resolve field value
	key := strings.TrimPrefix(field, "args.")
	actual := fmt.Sprintf("%v", args[key])

	match := actual == expected
	if op == "!==" {
		match = !match
	}

	if match {
		return trueVal
	}
	return falseVal
}

func resolveNestedValue(obj map[string]any, path string) any {
	parts := strings.Split(path, ".")
	var current any = obj
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = m[part]
		if !ok {
			return nil
		}
	}
	return current
}

func isTruthy(val any) bool {
	if val == nil {
		return false
	}
	switch v := val.(type) {
	case bool:
		return v
	case string:
		return v != "" && v != "false" && v != "0"
	case float64:
		return v != 0
	case int:
		return v != 0
	}
	return true
}

func pipelineLimit(data any, limitStr string) (any, error) {
	items, ok := toSlice(data)
	if !ok {
		return data, nil
	}

	limitStr = strings.TrimSpace(limitStr)

	// Try parsing as integer directly
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		// Try extracting first number from JS expression like "Math.min(30, 50)"
		numRe := regexp.MustCompile(`\d+`)
		nums := numRe.FindAllString(limitStr, -1)
		if len(nums) > 0 {
			// Use the smallest number as a safe limit
			limit = len(items)
			for _, n := range nums {
				if v, e := strconv.Atoi(n); e == nil && v < limit {
					limit = v
				}
			}
		} else {
			return data, nil
		}
	}

	if limit > len(items) {
		limit = len(items)
	}
	return items[:limit], nil
}

// renderTemplate replaces ${{ expr }} placeholders.
// Supports: args.X, item.X, index, index + 1
func renderTemplate(tmpl string, args map[string]any, item map[string]any, index int) string {
	re := regexp.MustCompile(`\$\{\{\s*(.+?)\s*\}\}`)
	return re.ReplaceAllStringFunc(tmpl, func(match string) string {
		expr := re.FindStringSubmatch(match)[1]

		// index + N
		if strings.HasPrefix(expr, "index") {
			rest := strings.TrimPrefix(expr, "index")
			rest = strings.TrimSpace(rest)
			if rest == "" {
				return strconv.Itoa(index)
			}
			if strings.HasPrefix(rest, "+") {
				n, _ := strconv.Atoi(strings.TrimSpace(rest[1:]))
				return strconv.Itoa(index + n)
			}
			return strconv.Itoa(index)
		}

		// args.X
		if strings.HasPrefix(expr, "args.") {
			// Handle ternary: args.sort === 'date' ? 'search_by_date' : 'search'
			if strings.Contains(expr, "?") {
				return evalSimpleTernary(expr, args)
			}
			// Handle pipe filters: args.name | json → JSON-encode the value
			key := strings.TrimPrefix(expr, "args.")
			filter := ""
			if pipeIdx := strings.Index(key, "|"); pipeIdx >= 0 {
				filter = strings.TrimSpace(key[pipeIdx+1:])
				key = strings.TrimSpace(key[:pipeIdx])
			}
			if v, ok := args[key]; ok {
				val := fmt.Sprintf("%v", v)
				if filter == "json" {
					b, _ := json.Marshal(val)
					return string(b)
				}
				return val
			}
			if filter == "json" {
				return `""`
			}
			return ""
		}

		// item.X (nested dot access)
		if strings.HasPrefix(expr, "item.") && item != nil {
			path := strings.TrimPrefix(expr, "item.")
			return resolveNestedField(item, path)
		}

		return match
	})
}

func resolveNestedField(obj map[string]any, path string) string {
	parts := strings.Split(path, ".")
	var current any = obj
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current, ok = m[part]
		if !ok {
			return ""
		}
	}
	// Format numbers without scientific notation
	if f, ok := current.(float64); ok {
		if f == float64(int64(f)) {
			return strconv.FormatInt(int64(f), 10)
		}
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return fmt.Sprintf("%v", current)
}

func toSlice(data any) ([]any, bool) {
	switch v := data.(type) {
	case []any:
		return v, true
	case []map[string]any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = item
		}
		return result, true
	}
	return nil, false
}

func httpGet(ctx context.Context, url string) (any, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	return doRequest(req)
}

func httpGetWithParams(ctx context.Context, url string, params map[string]any) (any, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	for k, v := range params {
		q.Set(k, fmt.Sprintf("%v", v))
	}
	req.URL.RawQuery = q.Encode()
	return doRequest(req)
}

func httpPost(ctx context.Context, url string, body map[string]any) (any, error) {
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return doRequest(req)
}

// executeWithBridge runs the pipeline using the AnyClaw browser extension bridge.
func (a *PipelineAdapter) executeWithBridge(ctx context.Context, cmd *pkg.Command, params map[string]any) (*Result, error) {
	var data any

	for _, step := range cmd.Pipeline {
		var err error
		for op, arg := range step {
			switch op {
			case "navigate":
				url := renderTemplate(fmt.Sprintf("%v", arg), params, nil, 0)
				err = BridgeNavigate(url)
			case "evaluate":
				script := renderTemplate(fmt.Sprintf("%v", arg), params, nil, 0)
				data, err = BridgeEvaluate(script)
			case "fetch":
				data, err = pipelineFetch(ctx, arg, params, data)
			case "select":
				data, err = pipelineSelect(data, renderTemplate(fmt.Sprintf("%v", arg), params, nil, 0))
			case "map":
				data, err = pipelineMap(data, arg, params)
			case "filter":
				data, err = pipelineFilter(data, fmt.Sprintf("%v", arg), params)
			case "limit":
				data, err = pipelineLimit(data, renderTemplate(fmt.Sprintf("%v", arg), params, nil, 0))
			}
			if err != nil {
				return nil, fmt.Errorf("pipeline step %q: %w", op, err)
			}
		}
	}

	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, err
	}
	var dataMap map[string]any
	json.Unmarshal(out, &dataMap)
	return &Result{Content: string(out), Data: dataMap}, nil
}

// pipelineNeedsBrowser checks if any step in the pipeline requires a browser.
func pipelineNeedsBrowser(pipeline []map[string]any) bool {
	for _, step := range pipeline {
		for op := range step {
			if op == "evaluate" {
				return true
			}
		}
	}
	return false
}

func doRequest(req *http.Request) (any, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result any
	if err := json.Unmarshal(body, &result); err != nil {
		return string(body), nil
	}
	return result, nil
}
