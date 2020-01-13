package metrics

import (
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type Logger interface {
	Log(info *MetricsInfo)
}

type StdoutLogger struct{}

func NewStdoutLogger() *StdoutLogger {
	return &StdoutLogger{}
}

func (l *StdoutLogger) Log(info *MetricsInfo) {
	infoStr, err := info.ToJSON()
	if err == nil {
		log.Print(infoStr)
	} else {
		log.Printf("StdoutLogger: error: %v", err)
	}
}

const defaultLogWriters = 2
const defaultQueueSize = 1000 * defaultLogWriters
const defaultMaxLogFileSize = 1024 * 1024 * 1024
const defaultMaxLogFiles = 10
const defaultLogFilePattern = "%d.log"

type FileLogger struct {
	MetricsQueue   chan *MetricsInfo
	LogDir         string
	MaxLogFileSize int64
	MaxLogFiles    int
	Verbose        bool
}

func NewFileLogger(logDir string, maxLogFileSize int64, maxLogFiles int, verbose bool) *FileLogger {
	if maxLogFileSize <= 0 {
		maxLogFileSize = defaultMaxLogFileSize
	}
	if maxLogFiles <= 0 {
		maxLogFiles = defaultMaxLogFiles
	}
	logger := &FileLogger{
		MetricsQueue:   make(chan *MetricsInfo, defaultQueueSize),
		LogDir:         logDir,
		MaxLogFileSize: maxLogFileSize,
		MaxLogFiles:    maxLogFiles,
		Verbose:        verbose,
	}

	for i := 0; i < defaultLogWriters; i++ {
		go logger.startLogWriter(i)
	}

	return logger
}

func (l *FileLogger) Log(info *MetricsInfo) {
	select {
	case l.MetricsQueue <- info:
	default:
		log.Printf("FileLogger: queue is full")
	}
}

func (l *FileLogger) startLogWriter(idx int) {
	f, err := l.openLogFile(idx)
	if err != nil {
		log.Printf("FileLogger%d: log open error: %v", idx, err)
	}

	for info := range l.MetricsQueue {
		infoStr, err := info.ToJSON()
		if err == nil {
			f, err = l.tryRotateLogFile(f, idx)
			if err != nil {
				continue
			}

			_, err := f.WriteString(infoStr)
			if err != nil {
				log.Printf("FileLogger%d: write error: %v", idx, err)
				continue
			}
			f.Sync()
		} else {
			log.Printf("FileLogger%d: info.ToJSON() error: %v", idx, err)
		}
	}
}

func (l *FileLogger) openLogFile(idx int) (*os.File, error) {
	logFilePath := path.Join(l.LogDir, fmt.Sprintf(defaultLogFilePattern, idx))
	return os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
}

func (l *FileLogger) tryRotateLogFile(currFile *os.File, idx int) (*os.File, error) {
	var info os.FileInfo
	var err error
	if info, err = os.Stat(currFile.Name()); os.IsNotExist(err) {
		newFile, fErr := l.openLogFile(idx)
		if fErr != nil {
			log.Printf("FileLogger%d: log open error: %v", idx, fErr)
			return currFile, nil
		}
		currFile.Close()
		return newFile, nil
	}

	if info.Size() < l.MaxLogFileSize {
		return currFile, nil
	}

	currLogFilePath := path.Join(l.LogDir, fmt.Sprintf(defaultLogFilePattern, idx))
	var rotatedLogFilePath string
	for i := 0; i < l.MaxLogFiles; i++ {
		filePath := path.Join(l.LogDir, fmt.Sprintf(defaultLogFilePattern+".%d", idx, i))
		if _, err := os.Stat(filePath + ".gz"); os.IsNotExist(err) {
			rotatedLogFilePath = filePath
			break
		}
	}

	if len(rotatedLogFilePath) == 0 {
		files, err := ioutil.ReadDir(l.LogDir)
		if err != nil {
			log.Printf("FileLogger%d: log rotation error: %v", idx, err)
			return currFile, nil
		}

		var oldestFileName string
		oldestTime := time.Now()
		for _, file := range files {
			if !file.Mode().IsRegular() {
				continue
			}

			fileName := filepath.Base(file.Name())
			baseFileName := strings.TrimSuffix(fileName, ".gz")
			fn := strings.TrimSuffix(baseFileName, path.Ext(baseFileName))

			if fn != fmt.Sprintf(defaultLogFilePattern, idx) {
				continue
			}

			if file.ModTime().Before(oldestTime) {
				oldestFileName = baseFileName
				oldestTime = file.ModTime()
			}
		}

		if len(oldestFileName) > 0 {
			rotatedLogFilePath = path.Join(l.LogDir, oldestFileName)
		} else {
			rotatedLogFilePath = path.Join(l.LogDir, fmt.Sprintf(defaultLogFilePattern+".%d", idx, 0))
		}

		if l.Verbose {
			log.Printf("FileLogger%d: maximum number of log files reached, overwriting %s", idx, rotatedLogFilePath)
		}
		err = os.Remove(rotatedLogFilePath + ".gz")
		if err != nil {
			log.Printf("FileLogger%d log rotation error: %v", idx, err)
			return currFile, nil
		}
	}

	currFile.Sync()
	err = l.compressLogFile(currLogFilePath, rotatedLogFilePath)
	if err != nil {
		log.Printf("FileLogger%d: log compression error: %v", idx, err)
		return currFile, nil
	}

	currFile.Truncate(0)
	currFile.Seek(0, 0)

	if l.Verbose {
		log.Printf("FileLogger%d: log file rotated: %v", idx, rotatedLogFilePath)
	}

	return currFile, nil
}

func (l *FileLogger) compressLogFile(currLogFilePath string, rotatedLogFilePath string) error {
	logFile, err := os.OpenFile(currLogFilePath, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer logFile.Close()

	compressedLogFilePath := rotatedLogFilePath + ".gz"
	compressedLogFile, err := os.OpenFile(compressedLogFilePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	zipWriter := gzip.NewWriter(compressedLogFile)

	_, err = io.Copy(zipWriter, logFile)
	zipWriter.Close()

	return err
}
