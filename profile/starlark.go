package profile

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	boxenerrors "github.com/carlmontanari/boxen/errors"
	starlarkjson "go.starlark.net/lib/json"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
	"go.yaml.in/yaml/v4"
)

type starlarkModule struct {
	globals starlark.StringDict
	err     error
}

func starlarkEnvironment(isPackaging bool) (*starlark.Thread, starlark.StringDict) {
	thread := &starlark.Thread{Name: "profile"}
	predeclared := starlark.StringDict{
		"read_file":    starlark.NewBuiltin("read_file", starlarkReadFile),
		"is_packaging": starlark.Bool(isPackaging),
		"json":         starlarkjson.Module,
	}
	cache := make(map[string]*starlarkModule)
	thread.Load = func(t *starlark.Thread, path string) (starlark.StringDict, error) {
		if !filepath.IsAbs(path) && t.CallStackDepth() > 0 {
			path = filepath.Join(filepath.Dir(t.CallFrame(0).Pos.Filename()), path)
		}
		path, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		if cached, ok := cache[path]; ok {
			if cached == nil {
				return nil, fmt.Errorf("%w: Starlark load cycle at %s", boxenerrors.ErrBoxen, path)
			}

			return cached.globals, cached.err
		}
		cache[path] = nil
		globals, err := starlark.ExecFileOptions(
			syntax.LegacyFileOptions(), t, path, nil, predeclared,
		)
		cache[path] = &starlarkModule{globals: globals, err: err}

		return globals, err
	}

	return thread, predeclared
}

//nolint:ireturn // Starlark's builtin callback signature requires a Value result.
func starlarkReadFile(
	_ *starlark.Thread,
	_ *starlark.Builtin,
	args starlark.Tuple,
	kwargs []starlark.Tuple,
) (starlark.Value, error) {
	var path string
	var fallback starlark.Value
	if err := starlark.UnpackArgs("read_file", args, kwargs,
		"path", &path, "default?", &fallback); err != nil {
		return nil, err
	}
	var defaults []string
	if fallback != nil {
		value, ok := starlark.AsString(fallback)
		if !ok {
			return nil, fmt.Errorf("%w: read_file default must be a string", boxenerrors.ErrBoxen)
		}
		defaults = []string{value}
	}
	content, err := readFile(path, defaults...)
	if err != nil {
		return nil, err
	}

	return starlark.String(content), nil
}

// callStarlark accepts inline source or nil to read filename from disk. Arguments
// and results use JSON-compatible values, preserving integers through YAML decoding.
func callStarlark(
	source any,
	filename, function string,
	isPackaging bool,
	args ...any,
) (any, error) {
	thread, predeclared := starlarkEnvironment(isPackaging)
	globals, err := starlark.ExecFileOptions(
		syntax.LegacyFileOptions(), thread, filename, source, predeclared,
	)
	if err != nil {
		return nil, err
	}
	f, ok := globals[function]
	if !ok {
		return nil, fmt.Errorf(
			"%w: %s does not define %s",
			boxenerrors.ErrBoxen,
			filename,
			function,
		)
	}
	values := make(starlark.Tuple, len(args))
	for i, arg := range args {
		encoded, marshalErr := json.Marshal(arg)
		if marshalErr != nil {
			return nil, marshalErr
		}
		values[i], err = starlark.Call(thread, starlarkjson.Module.Members["decode"],
			starlark.Tuple{starlark.String(encoded)}, nil)
		if err != nil {
			return nil, err
		}
	}
	result, err := starlark.Call(thread, f, values, nil)
	if err != nil {
		return nil, err
	}
	encoded, err := starlark.Call(thread, starlarkjson.Module.Members["encode"],
		starlark.Tuple{result}, nil)
	if err != nil {
		return nil, err
	}
	encodedText, ok := starlark.AsString(encoded)
	if !ok {
		return nil, fmt.Errorf("%w: JSON encoder did not return a string", boxenerrors.ErrBoxen)
	}
	var out any
	if err := yaml.Unmarshal([]byte(encodedText), &out); err != nil {
		return nil, err
	}

	return out, nil
}
