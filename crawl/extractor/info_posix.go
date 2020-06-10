package extractor

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	goeval "github.com/edisonguo/govaluate"
)

func ExtractPosix(rootDir string, conc int, pattern string, followSymlink bool, outputFormat string) error {
	absRootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return err
	}

	expr, err := parsePatternExpression(pattern)
	if err != nil {
		return err
	}

	crawler := NewPosixCrawler(conc, expr, followSymlink, outputFormat)
	err = crawler.Crawl(absRootDir)
	if err != nil {
		os.Stderr.Write([]byte(err.Error() + "\n"))
	}
	return nil
}

func parsePatternExpression(pattern string) (*goeval.EvaluableExpression, error) {
	if len(strings.TrimSpace(pattern)) == 0 {
		return nil, nil
	}

	expr, err := goeval.NewEvaluableExpression(pattern)
	if err != nil {
		return nil, err
	}

	validVariables := map[string]struct{}{"path": struct{}{}, "type": struct{}{}}
	for _, token := range expr.Tokens() {
		if token.Kind == goeval.VARIABLE {
			varName, ok := token.Value.(string)
			if !ok {
				return nil, fmt.Errorf("variable token '%v' failed to cast string", token.Value)
			}
			if _, found := validVariables[varName]; !found {
				return nil, fmt.Errorf("variable %v is not supported. Valid variables are %v", varName, validVariables)
			}
		}
	}
	return expr, nil
}

const DefaultMaxPosixErrors = 1000

type PosixCrawler struct {
	SubDirs       chan string
	Outputs       chan *PosixInfo
	Error         chan error
	wg            sync.WaitGroup
	concLimit     chan struct{}
	outputDone    chan struct{}
	pattern       *goeval.EvaluableExpression
	followSymlink bool
	outputFormat  string
}

func NewPosixCrawler(conc int, pattern *goeval.EvaluableExpression, followSymlink bool, outputFormat string) *PosixCrawler {
	crawler := &PosixCrawler{
		SubDirs:       make(chan string, 4096),
		Outputs:       make(chan *PosixInfo, 4096),
		Error:         make(chan error, 100),
		wg:            sync.WaitGroup{},
		concLimit:     make(chan struct{}, conc),
		outputDone:    make(chan struct{}, 1),
		pattern:       pattern,
		followSymlink: followSymlink,
		outputFormat:  outputFormat,
	}
	return crawler
}

func (pc *PosixCrawler) Crawl(currPath string) error {
	go pc.outputResult()

	pc.wg.Add(1)
	pc.concLimit <- struct{}{}
	pc.crawlDir(currPath)
	pc.wg.Wait()

	close(pc.Outputs)
	<-pc.outputDone

	close(pc.Error)
	var errors []string
	errCount := 0
	for err := range pc.Error {
		errors = append(errors, err.Error())
		errCount++
		if errCount >= DefaultMaxPosixErrors {
			errors = append(errors, " ... too many errors")
			break
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf(strings.Join(errors, "\n"))
	}

	return nil
}

func (pc *PosixCrawler) crawlDir(currPath string) {
	defer pc.wg.Done()
	defer func() { <-pc.concLimit }()
	files, err := readDir(currPath)
	if err != nil {
		select {
		case pc.Error <- err:
		default:
		}
		return
	}

	for _, fi := range files {
		fileName := fi.Name()
		filePath := path.Join(currPath, fileName)
		fileMode := fi.Mode()

		if pc.followSymlink && fileMode&os.ModeSymlink == os.ModeSymlink {
			newFi, err := pc.resolveSymlink(currPath, fileName)
			if err != nil {
				select {
				case pc.Error <- err:
				default:
				}
				continue
			}

			fi = newFi
			fileMode = fi.Mode()
		}

		validFileMode := fileMode.IsDir() || fileMode.IsRegular()
		if !validFileMode {
			continue
		}

		if pc.pattern != nil {
			result, err := pc.evaluatePatternExpression(filePath, fileMode)
			if err != nil {
				select {
				case pc.Error <- err:
				default:
				}
				continue
			}

			if !result {
				continue
			}
		}

		if fileMode.IsDir() {
			pc.wg.Add(1)
			go func(p string) {
				pc.concLimit <- struct{}{}
				pc.crawlDir(p)
			}(filePath)
			continue
		}

		stat := fi.Sys().(*syscall.Stat_t)
		fileSignature := fmt.Sprintf("%s%d%d%d%d", filePath, stat.Ino, stat.Size, stat.Mtim.Sec, stat.Mtim.Nsec)
		info := &PosixInfo{
			FilePath: filePath,
			INode:    stat.Ino,
			Size:     stat.Size,
			MTime:    time.Unix(int64(stat.Mtim.Sec), int64(stat.Mtim.Nsec)).UTC(),
			CTime:    time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec)).UTC(),
			ID:       fmt.Sprintf("%x", md5.Sum([]byte(fileSignature))),
		}
		pc.Outputs <- info
	}
}

func readDir(path string) ([]os.FileInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	list, err := f.Readdir(-1)
	f.Close()
	return list, err
}

func (pc *PosixCrawler) evaluatePatternExpression(filePath string, fileMode os.FileMode) (bool, error) {
	var fileType string
	if fileMode.IsDir() {
		fileType = "d"
	} else if fileMode.IsRegular() {
		fileType = "f"
	}

	parameters := map[string]interface{}{"type": fileType, "path": filePath}
	result, err := pc.pattern.Evaluate(parameters)
	if err != nil {
		return false, fmt.Errorf("pattern expression: %v", err)
	}

	val, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("pattern expression: result '%v' is not boolean", result)
	}
	return val, nil
}

func (pc *PosixCrawler) resolveSymlink(currPath string, linkName string) (os.FileInfo, error) {
	filePath := currPath
	linkName = path.Join(filePath, linkName)
	fileName, err := os.Readlink(linkName)
	if err != nil {
		return nil, err
	}
	if !path.IsAbs(fileName) {
		fileName = path.Join(filePath, fileName)
		fileName = filepath.Clean(fileName)
		filePath = filepath.Dir(fileName)
	}

	isSymlink := true
	filesSeen := make(map[string]struct{})

	for {
		fi, err := os.Lstat(fileName)
		if err != nil {
			return nil, err
		}

		if _, found := filesSeen[fileName]; found {
			return nil, fmt.Errorf("circular symlink: %v", linkName)
		}
		filesSeen[fileName] = struct{}{}

		isSymlink = fi.Mode()&os.ModeSymlink == os.ModeSymlink
		if isSymlink {
			fileName, err = os.Readlink(fileName)
			if err != nil {
				return nil, err
			}
			if !path.IsAbs(fileName) {
				fileName = path.Join(filePath, fileName)
				fileName = filepath.Clean(fileName)
				filePath = filepath.Dir(fileName)
			}
		} else {
			return fi, nil
		}
	}
}

func (pc *PosixCrawler) outputResult() {
	for info := range pc.Outputs {
		out, _ := json.Marshal(info)
		rec := string(out)
		if pc.outputFormat == "tsv" {
			rec = fmt.Sprintf("%s\tposix\t%s", info.FilePath, rec)
		}
		fmt.Printf("%s\n", rec)
	}
	pc.outputDone <- struct{}{}
}
