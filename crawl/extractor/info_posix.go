package extractor

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

func ExtractPosix(rootDir string, conc int, pattern string, followSymlink bool, outputFormat string) {
	absRootDir, err := filepath.Abs(rootDir)
	if err != nil {
		os.Stderr.Write([]byte(err.Error() + "\n"))
		return
	}
	crawler := NewPosixCrawler(conc, pattern, followSymlink, outputFormat)
	err = crawler.Crawl(absRootDir)
	if err != nil {
		os.Stderr.Write([]byte(err.Error() + "\n"))
	}
}

const DefaultMaxPosixErrors = 1000

type PosixCrawler struct {
	SubDirs       chan string
	Outputs       chan *PosixInfo
	Error         chan error
	wg            sync.WaitGroup
	concLimit     chan bool
	pattern       *regexp.Regexp
	followSymlink bool
	outputFormat  string
}

func NewPosixCrawler(conc int, pattern string, followSymlink bool, outputFormat string) *PosixCrawler {
	crawler := &PosixCrawler{
		SubDirs:       make(chan string, 4096),
		Outputs:       make(chan *PosixInfo, 4096),
		Error:         make(chan error, 100),
		wg:            sync.WaitGroup{},
		concLimit:     make(chan bool, conc),
		followSymlink: followSymlink,
		outputFormat:  outputFormat,
	}

	if len(strings.TrimSpace(pattern)) > 0 {
		crawler.pattern = regexp.MustCompile(pattern)
	}

	return crawler
}

func (pc *PosixCrawler) Crawl(currPath string) error {
	go pc.outputResult()

	pc.wg.Add(1)
	pc.concLimit <- false
	pc.crawlDir(currPath)
	pc.wg.Wait()

	close(pc.Outputs)
	pc.outputResult()

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

		if pc.followSymlink && (fileMode&os.ModeSymlink == os.ModeSymlink) {
			newFi, newPath, err := pc.resolveSymlink(currPath, fileName)
			if err != nil {
				select {
				case pc.Error <- err:
				default:
				}
				continue
			}

			fi = newFi
			fileName = fi.Name()
			filePath = path.Join(newPath, fileName)
			fileMode = fi.Mode()
		}

		if fileMode.IsDir() {
			pc.wg.Add(1)
			go func(p string) {
				pc.concLimit <- false
				pc.crawlDir(p)
			}(filePath)
			continue
		}

		if !fileMode.IsRegular() {
			continue
		}

		if pc.pattern != nil && !pc.pattern.MatchString(filePath) {
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

func (pc *PosixCrawler) resolveSymlink(currPath string, linkName string) (os.FileInfo, string, error) {
	filePath := currPath
	linkName = path.Join(filePath, linkName)
	fileName, err := os.Readlink(linkName)
	if err != nil {
		return nil, "", err
	}
	if !path.IsAbs(fileName) {
		fileName = path.Join(filePath, fileName)
		fileName = filepath.Clean(fileName)
		filePath = filepath.Dir(fileName)
	}

	isSymlink := true
	filesSeen := make(map[string]bool)

	for {
		fi, err := os.Lstat(fileName)
		if err != nil {
			return nil, "", err
		}

		if _, found := filesSeen[fileName]; found {
			return nil, "", fmt.Errorf("circular symlink: %v", linkName)
		}
		filesSeen[fileName] = false

		isSymlink = fi.Mode()&os.ModeSymlink == os.ModeSymlink
		if isSymlink {
			fileName, err = os.Readlink(fileName)
			if err != nil {
				return nil, "", err
			}
			if !path.IsAbs(fileName) {
				fileName = path.Join(filePath, fileName)
				fileName = filepath.Clean(fileName)
				filePath = filepath.Dir(fileName)
			}
			continue
		} else {
			return fi, filePath, nil
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
		fmt.Printf("%s\n", string(out))
	}
}
