package transform

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

// Max execution times for a single transformer
const MAX_TIMES = 10

const TMP_DIR = ".tftmp"
const BAK_DIR = ".bak"
const LOG_FILE = ".tflog"
const TF_PREFIX = ".tf."

// ".*", "*.aria2", "desktop.ini", "Thumbs.db"
var IgnoredFilePatterns = []string{
	".*",
	"*.aria2", // aria2 control files
	"desktop.ini",
	"Thumbs.db",
}

var (
	ErrInvalid = fmt.Errorf("invalid contents")
)

// Transform filenames and / or contents of dir.
// If transformer made any modification to file system, changed should be set to true.
// Transformer MUST be finally idempotent, that is, if an invocation returns (false, nil),
// any successive invocations of the same transformer should return the same result.
type Transformer struct {
	Name   string
	Action func(tc *TransformerContext) (changed bool, err error)
}

type TransformerContext struct {
	CurrentTransformer *Transformer
	Dir                string
	BackupDir          string
	Changed            bool
	Err                error // error processing file
	Options            url.Values
	log                *os.File // opened *os.File of LogFile
}

type TransformStep struct {
	Transformers []*Transformer
	// At most execute Transformers these times. -1 == unlimited.
	// If the final one of Transformers return changed=false, it will stop further invocations.
	Times int
}

type Transformers []*TransformStep

type Logger func(format string, v ...any)

var (
	allTransformers = map[string]*Transformer{}
)

// Normalizer a dir.
// It's idempotent. Successive invocation will have output.Changed == false and output.Err == nil.
func (ts Transformers) Transform(dir string, options url.Values) (output *TransformerContext) {
	bakDir := options.Get("bakdir")
	if bakDir == "" {
		bakDir = filepath.Join(dir, BAK_DIR)
	}
	tc := &TransformerContext{
		Dir:       dir,
		BackupDir: bakDir,
		Options:   options,
	}
	if err := os.MkdirAll(bakDir, 0700); err != nil {
		tc.Err = err
		return
	}
	log, err := os.OpenFile(filepath.Join(bakDir, LOG_FILE), os.O_SYNC|os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		tc.Err = fmt.Errorf("failed to open log file: %w", err)
		return tc
	}
	defer log.Close()
	tc.log = log
	tc.Log("[dir %q]Start transforms", dir)
	defer tc.Log("[dir %q]All transforms completed, changed=%t, err=%v", dir, tc.Changed, tc.Err)
main:
	for _, step := range ts {
		i := 0
		for {
			for j, transformer := range step.Transformers {
				tc.CurrentTransformer = transformer
				tc.Log("Start")
				changed, err := transformer.Action(tc)
				if changed {
					tc.Changed = true
				}
				tc.Log("Finished with changed=%t, err=%v", changed, err)
				if err != nil {
					tc.Err = fmt.Errorf("[transformer %s]%w", transformer.Name, err)
					break main
				}
				if !changed && j == len(step.Transformers)-1 {
					continue main
				}
			}
			i++
			if step.Times >= 0 && i >= step.Times {
				continue main
			} else if i >= MAX_TIMES {
				tc.Err = fmt.Errorf("loop too many times")
				break main
			}
		}
	}
	tc.CurrentTransformer = nil
	return tc
}

// Log() should be called before action.
func (tc *TransformerContext) Log(format string, v ...any) {
	msg := ""
	if tc.CurrentTransformer != nil {
		msg += fmt.Sprintf("[Transformer %s]", tc.CurrentTransformer.Name)
	}
	msg += fmt.Sprintf(format, v...)
	log.Tracef(msg)
	tc.log.WriteString(msg)
	if !strings.HasSuffix(msg, "\n") {
		tc.log.WriteString("\n")
	}
}

// Each arg: transformerName (string), transformerNames ([]string / []any(.string)), times (int / string).
// times is the value of previous transformerName or transformerNames.
// In the case of times omitted, it's assumed to be 1.
// E.g. NewNormalizer("a", ["b", "c"], -1, "d", 2): "a" => "b+c" * N => "d" * 2.
func NewNormalizer(args ...any) (Transformers, error) {
	var transformSteps Transformers
	var currentStep *TransformStep
	for i := range args {
		switch arg := args[i].(type) {
		case string:
			// integer string
			if arg[0] == '-' || (arg[0] >= '0' && arg[0] <= '9') {
				times, err := strconv.Atoi(arg)
				if err != nil {
					return nil, fmt.Errorf("invalid arg %v (string starts with - or number is parsed as integer)", arg)
				}
				if currentStep == nil {
					return nil, fmt.Errorf("invalid arg %v: times must follow a name or names", arg)
				}
				currentStep.Times = times
				transformSteps = append(transformSteps, currentStep)
				currentStep = nil
				break
			}
			if currentStep != nil {
				transformSteps = append(transformSteps, currentStep)
			}
			if allTransformers[arg] == nil {
				return nil, fmt.Errorf("transformer %s not exists", arg)
			}
			currentStep = &TransformStep{
				Transformers: []*Transformer{allTransformers[arg]},
			}
		case []any:
			if currentStep != nil {
				transformSteps = append(transformSteps, currentStep)
			}
			var transformers []*Transformer
			for _, a := range arg {
				if name, ok := a.(string); !ok {
					return nil, fmt.Errorf("invalid arg: param %v in arg %v type unsupported", a, arg)
				} else {
					if allTransformers[name] == nil {
						return nil, fmt.Errorf("transformer %s not exists", name)
					}
					transformers = append(transformers, allTransformers[name])
				}
			}
			currentStep = &TransformStep{
				Transformers: transformers,
			}
		case []string:
			if currentStep != nil {
				transformSteps = append(transformSteps, currentStep)
			}
			var transformers []*Transformer
			for _, name := range arg {
				if allTransformers[name] == nil {
					return nil, fmt.Errorf("transformer %s not exists", name)
				}
				transformers = append(transformers, allTransformers[name])
			}
			currentStep = &TransformStep{
				Transformers: transformers,
			}
		case int:
			if currentStep == nil {
				return nil, fmt.Errorf("invalid arg %v: times must follow a name or names", arg)
			}
			currentStep.Times = arg
			transformSteps = append(transformSteps, currentStep)
			currentStep = nil
		default:
			return nil, fmt.Errorf("invalid args: arg %v type unsupported", arg)
		}
	}
	if currentStep != nil {
		transformSteps = append(transformSteps, currentStep)
	}
	if len(transformSteps) == 0 {
		return nil, fmt.Errorf("no transformers")
	}
	return transformSteps, nil
}

func Register(transformer *Transformer) {
	allTransformers[transformer.Name] = transformer
}
