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
	"unsafe"

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

func GetPosixInfo(filePath string, fStat os.FileInfo) *PosixInfo {
	stat := fStat.Sys().(*syscall.Stat_t)
	fileSignature := fmt.Sprintf("%s%d%d%d%d", filePath, stat.Ino, stat.Size, stat.Mtim.Sec, stat.Mtim.Nsec)
	return &PosixInfo{
		FilePath: filePath,
		INode:    stat.Ino,
		Size:     stat.Size,
		MTime:    time.Unix(int64(stat.Mtim.Sec), int64(stat.Mtim.Nsec)).UTC(),
		CTime:    time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec)).UTC(),
		ID:       fmt.Sprintf("%x", md5.Sum([]byte(fileSignature))),
	}
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

type DirEntInfo struct {
	Name string
	Mode uint8
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
	pc.crawlDir(currPath, false)
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

func (pc *PosixCrawler) crawlDir(currPath string, serialised bool) {
	defer pc.wg.Done()
	if !serialised {
		defer func() { <-pc.concLimit }()
	}
	files, err := readDir(currPath)
	if err != nil {
		select {
		case pc.Error <- err:
		default:
		}
		return
	}

	for _, fi := range files {
		fileName := fi.Name
		filePath := path.Join(currPath, fileName)
		fileMode := fi.Mode

		var fStat os.FileInfo
		if fileMode == syscall.DT_LNK {
			if !pc.followSymlink {
				continue
			}
			fStat, err = os.Stat(filePath)
			if err != nil {
				select {
				case pc.Error <- err:
				default:
				}
				continue
			}

			fMode := fStat.Mode()
			if fMode.IsDir() {
				fileMode = syscall.DT_DIR
			} else if fMode.IsRegular() {
				fileMode = syscall.DT_REG
			}
		}

		validFileMode := fileMode == syscall.DT_DIR || fileMode == syscall.DT_REG
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

		if fileMode == syscall.DT_DIR {
			pc.wg.Add(1)
			select {
			case pc.concLimit <- struct{}{}:
				go func(p string) {
					pc.crawlDir(p, false)
				}(filePath)
			default:
				pc.crawlDir(filePath, true)
			}
			continue
		}

		if fStat == nil {
			fStat, err = os.Lstat(filePath)
			if err != nil {
				select {
				case pc.Error <- err:
				default:
				}
				continue
			}
		}

		info := GetPosixInfo(filePath, fStat)
		/*
			stat := fStat.Sys().(*syscall.Stat_t)
			fileSignature := fmt.Sprintf("%s%d%d%d%d", filePath, stat.Ino, stat.Size, stat.Mtim.Sec, stat.Mtim.Nsec)
			info := &PosixInfo{
				FilePath: filePath,
				INode:    stat.Ino,
				Size:     stat.Size,
				MTime:    time.Unix(int64(stat.Mtim.Sec), int64(stat.Mtim.Nsec)).UTC(),
				CTime:    time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec)).UTC(),
				ID:       fmt.Sprintf("%x", md5.Sum([]byte(fileSignature))),
			}
		*/
		pc.Outputs <- info
	}
}

func readDir(currDir string) ([]DirEntInfo, error) {
	parentDir := filepath.Dir(currDir)

	dhParent, err := os.Open(parentDir)
	if err != nil {
		return nil, fmt.Errorf("Could not open dir: %s", err.Error())
	}
	defer dhParent.Close()
	dirFd := int(dhParent.Fd())

	file := filepath.Base(currDir)

	dh, err := syscall.Openat(dirFd, file, syscall.O_RDONLY, 0777)
	if err != nil {
		return nil, fmt.Errorf("Could not open %s: %s", currDir, err.Error())
	}
	defer syscall.Close(dh)

	origBuf := make([]byte, 4096)
	var entries []DirEntInfo
	for {
		n, errno := syscall.ReadDirent(dh, origBuf)
		if errno != nil {
			return nil, fmt.Errorf("Could not read dirent: %v", errno)
		}
		if n <= 0 {
			break
		}

		buf := origBuf[0:n]
		for len(buf) > 0 {
			dirent := (*syscall.Dirent)(unsafe.Pointer(&buf[0]))
			buf = buf[dirent.Reclen:]
			if dirent.Ino == 0 {
				continue
			}
			ii := 0
			for ; ii < len(dirent.Name); ii++ {
				if dirent.Name[ii] == 0 {
					break
				}
			}
			bytes := (*[256]byte)(unsafe.Pointer(&dirent.Name[0]))
			name := string(bytes[:][:ii])
			if name == "." || name == ".." {
				continue
			}

			if dirent.Type == syscall.DT_UNKNOWN {
				st, err := os.Lstat(path.Join(currDir, name))
				if err != nil {
					return nil, err
				}
				mode := st.Mode()
				if mode.IsDir() {
					dirent.Type = syscall.DT_DIR
				} else if mode.IsRegular() {
					dirent.Type = syscall.DT_REG
				} else if mode&os.ModeSymlink == os.ModeSymlink {
					dirent.Type = syscall.DT_LNK
				}
			}

			info := DirEntInfo{Name: name, Mode: dirent.Type}
			entries = append(entries, info)
		}
	}
	return entries, nil
}

func (pc *PosixCrawler) evaluatePatternExpression(filePath string, fileMode uint8) (bool, error) {
	var fileType string
	if fileMode == syscall.DT_DIR {
		fileType = "d"
	} else if fileMode == syscall.DT_REG {
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
