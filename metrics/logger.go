package metrics

import (
	"fmt"
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

const defaultQueueSize = 2000
const defaultLogWriters = 2
const defaultMaxLogFileSize = 1024 * 1024 * 1024
const defaultMaxLogFiles = 10

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
	l.MetricsQueue <- info
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
	logFilePath := path.Join(l.LogDir, fmt.Sprintf("log%d", idx))
	return os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
}

func (l *FileLogger) tryRotateLogFile(currFile *os.File, idx int) (*os.File, error) {
	info, err := currFile.Stat()
	if err != nil {
		log.Printf("FileLogger%d: log rotation error: %v", idx, err)
		return currFile, nil
	}

	if info.Size() >= l.MaxLogFileSize {
		currLogFilePath := path.Join(l.LogDir, fmt.Sprintf("log%d", idx))
		var rotatedLogFilePath string
		for i := 0; i < l.MaxLogFiles; i++ {
			filePath := path.Join(l.LogDir, fmt.Sprintf("log%d.%d", idx, i))
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
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

			var oldestFile os.FileInfo
			oldestTime := time.Now()
			for _, file := range files {
				if !file.Mode().IsRegular() {
					continue
				}

				fileName := filepath.Base(file.Name())
				fn := strings.TrimSuffix(fileName, path.Ext(fileName))

				if fn != fmt.Sprintf("log%d", idx) {
					continue
				}

				if file.ModTime().Before(oldestTime) {
					oldestFile = file
					oldestTime = file.ModTime()
				}
			}

			if oldestFile != nil {
				rotatedLogFilePath = path.Join(l.LogDir, oldestFile.Name())
			} else {
				rotatedLogFilePath = path.Join(l.LogDir, fmt.Sprintf("log%d.%d", idx, 0))
			}

			if l.Verbose {
				log.Printf("FileLogger%d: maximum number of log files reached, overwriting %s", idx, rotatedLogFilePath)
			}
			err = os.Remove(rotatedLogFilePath)
			if err != nil {
				log.Printf("FileLogger%d log rotation error: %v", idx, err)
				return currFile, nil
			}
		}

		currFile.Close()
		err := os.Rename(currLogFilePath, rotatedLogFilePath)
		if err != nil {
			log.Printf("FileLogger%d: log rotation error: %v", idx, err)
			return currFile, nil
		}

		if l.Verbose {
			log.Printf("FileLogger%d: log file rotated: %v", idx, rotatedLogFilePath)
		}
	}

	f, err := l.openLogFile(idx)
	if err != nil {
		log.Printf("FileLogger%d: log rotation error: %v", idx, err)
	}

	return f, err
}
