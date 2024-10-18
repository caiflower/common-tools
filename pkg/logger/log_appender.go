package logger

import (
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/caiflower/common-tools/pkg/syncx"
	"github.com/caiflower/common-tools/pkg/tools"
)

type Appender interface {
	write(data data)
	needCutLog() bool
	rollingCutLog() bool
	loadLoggerFiles()
	compressLog()
	cleanLog()
	close()
}

type logAppender struct {
	timeFormat    string
	isConsole     bool
	enableTrace   bool
	dir           string
	fileName      string
	rollingPolicy string
	maxTime       int64
	maxSize       int64

	bufPool      sync.Pool
	log          *log.Logger
	writeLock    sync.Locker
	compressLock sync.Locker
	logFile      *os.File
	filesize     int64
	lastTime     time.Time
	loggerFiles  []string
}

func newLogAppender(timeFormat, path, fileName, rollingPolicy, maxTime, maxSize string, enableTrace bool) Appender {
	appender := &logAppender{
		timeFormat: timeFormat,
		bufPool: sync.Pool{
			New: func() interface{} {
				return new(strings.Builder)
			}},
		dir:           path,
		fileName:      fileName,
		rollingPolicy: rollingPolicy,
		enableTrace:   enableTrace,
		writeLock:     syncx.NewSpinLock(),
		compressLock:  syncx.NewSpinLock(),
		log:           new(log.Logger),
		maxSize:       getMaxSize(maxSize),
		maxTime:       getMaxTime(maxTime),
	}

	if appender.maxSize == 0 && appender.maxTime == 0 {
		appender.rollingPolicy = ""
	}

	if appender.dir == "" {
		appender.isConsole = true
		appender.log.SetOutput(os.Stdout)
	} else {
		// 创建目录
		err := tools.Mkdir(appender.dir, 0666)
		if err != nil {
			panic(fmt.Sprintf("[logger appender] mkdir err: %s\n", err))
		}

		// 创建文件流
		appender.createFilestream()
		// 倒入现有的所有file
		appender.loadLoggerFiles()
	}

	return appender
}

func getMaxTime(maxTime string) int64 {
	if maxTime == "" {
		return 0
	}
	var num int
	var err error
	if strings.HasSuffix(maxTime, "h") {
		numStr := strings.Replace(maxTime, "h", "", 1)
		if num, err = strconv.Atoi(numStr); err != nil {
			panic(fmt.Sprintf("[logger appender] convert %s num err: %s", numStr, err.Error()))
		}
		num *= 60 * 60
	} else if strings.HasSuffix(maxTime, "s") {
		numStr := strings.Replace(maxTime, "s", "", 1)
		if num, err = strconv.Atoi(numStr); err != nil {
			panic(fmt.Sprintf("[logger appender] convert %s num err: %s", numStr, err.Error()))
		}
	} else if strings.HasSuffix(maxTime, "m") {
		numStr := strings.Replace(maxTime, "m", "", 1)
		if num, err = strconv.Atoi(numStr); err != nil {
			panic(fmt.Sprintf("[logger appender] convert %s num err: %s", numStr, err.Error()))
		}
		num *= 60
	} else {
		panic(fmt.Sprintf("[logger appender] maxTime unKnown %s", maxTime))
	}
	return int64(num)
}

func getMaxSize(maxSize string) int64 {
	if maxSize == "" {
		return 0
	}
	var num int
	var err error
	if strings.HasSuffix(maxSize, "KB") {
		numStr := strings.Replace(maxSize, "KB", "", 1)
		if num, err = strconv.Atoi(numStr); err != nil {
			panic(fmt.Sprintf("[logger appender] convert %s num err: %s", numStr, err.Error()))
		}
		num *= 1024
	} else if strings.HasSuffix(maxSize, "MB") {
		numStr := strings.Replace(maxSize, "MB", "", 1)
		if num, err = strconv.Atoi(numStr); err != nil {
			panic(fmt.Sprintf("[logger appender] convert %s num err: %s", numStr, err.Error()))
		}
		num *= 1024 * 1024
	} else if strings.HasSuffix(maxSize, "GB") {
		numStr := strings.Replace(maxSize, "GB", "", 1)
		if num, err = strconv.Atoi(numStr); err != nil {
			panic(fmt.Sprintf("[logger appender] convert %s num err: %s", numStr, err.Error()))
		}
		num *= 1024 * 1024 & 1024
	} else {
		panic(fmt.Sprintf("[logger appender] maxSize unKnown %s", maxSize))
	}
	return int64(num)
}

func (appender *logAppender) createFilestream() {
	if appender.logFile != nil {
		if err := appender.logFile.Close(); err != nil {
			fmt.Printf("[logger appender] close logfile err: %s\n", err)
		}
	}
	filePath := appender.dir + "/" + appender.fileName
	logfile, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		panic(fmt.Errorf("[logger appender] open logfile err: %s\n", err))
	}
	appender.logFile = logfile
	appender.filesize, err = tools.FileSize(filePath)
	if err != nil {
		fmt.Printf("[logger appender] get file size err: %s\n", err)
	}
	appender.lastTime, err = tools.FileModTime(filePath)
	if err != nil {
		fmt.Printf("[logger appender] get file lastTime err: %s\n", err)
	}
	appender.log.SetOutput(appender.logFile)
}

func (appender *logAppender) write(data data) {
	defer onError("[logger appender]")

	if needCompress := appender.rollingCutLog(); needCompress {
		go func() {
			appender.compressLog()
			appender.cleanLog()
		}()
	}

	buf := appender.bufPool.Get().(*strings.Builder)
	buf.Reset()
	buf.WriteString(data.timestamp.Format(appender.timeFormat))
	if appender.enableTrace && data.traceID != "" {
		buf.WriteString(" [")
		buf.WriteString(data.traceID)
		buf.WriteString("]")
	}
	buf.WriteString(" [")
	buf.WriteString(data.level)
	buf.WriteString("] ")
	buf.WriteString(data.position)
	buf.WriteString(" - ")
	buf.WriteString(data.content)

	// 输出日志
	appender.writeLock.Lock()
	defer func() {
		appender.writeLock.Unlock()
		buf.Reset()
		appender.bufPool.Put(buf)
	}()

	if appender.isConsole {
		appender.log.Println(buf.String())
	} else {
		if err := appender.log.Output(5, buf.String()); err != nil {
			fmt.Printf("[ERROR] - output err %s", err.Error())
		}
		appender.filesize += int64(buf.Len())
	}
}

func (appender *logAppender) rollingCutLog() bool {
	if appender.needCutLog() {
		appender.writeLock.Lock()
		defer appender.writeLock.Unlock()
		if appender.needCutLog() {
			appender.cutLog()
			return true
		}
	}
	return false
}

func (appender *logAppender) cutLog() {
	if appender.logFile != nil {
		if err := appender.logFile.Close(); err != nil {
			fmt.Printf("[logger appender] close logfile err: %s\n", err)
		}
		appender.logFile = nil
	}

	targetFileName := appender.fileName + "-" + time.Now().Format("20060102150405")
	i := 0
	for {
		if tools.StringSliceContains(appender.loggerFiles, targetFileName+"-"+strconv.Itoa(i)) {
			i++
		} else {
			targetFileName = targetFileName + "-" + strconv.Itoa(i)
			break
		}
	}

	targetFilePath := appender.dir + "/" + targetFileName
	if err := os.Rename(appender.dir+"/"+appender.fileName, targetFilePath); err != nil {
		fmt.Printf("[logger appender] rename logfile err: %s\n", err)
	}

	// 切换文件
	appender.createFilestream()
	appender.loggerFiles = append(appender.loggerFiles, targetFileName)
}

func (appender *logAppender) loadLoggerFiles() {
	appender.writeLock.Lock()
	appender.writeLock.Unlock()

	if dir, err := os.ReadDir(appender.dir); err != nil {
		panic(fmt.Sprintf("[logger appender] loadLoggerFiles err: %s\n", err))
	} else {
		for _, v := range dir {
			if !v.IsDir() {
				if name := v.Name(); strings.HasPrefix(name, appender.fileName) {
					appender.loggerFiles = append(appender.loggerFiles, name)
				}
			}
		}
	}

}

func (appender *logAppender) compressLog() {

}

func (appender *logAppender) cleanLog() {

}

func (appender *logAppender) needCutLog() bool {
	switch appender.rollingPolicy {
	case RollingPolicyTimeAndSize:
		return appender.lastTime.Add(time.Duration(appender.maxTime)*time.Second).Before(time.Now()) || appender.filesize > appender.maxSize
	case RollingPolicyTime:
		return appender.lastTime.Add(time.Duration(appender.maxTime) * time.Second).Before(time.Now())
	case RollingPolicySize:
		return appender.filesize > appender.maxSize
	case RollingPolicyClose:
		return false
	default:
		fmt.Printf("unknown rolling policy %s\n", appender.rollingPolicy)
	}
	return false
}

func (appender *logAppender) close() {
	appender.writeLock.Lock()
	defer appender.writeLock.Unlock()
	if appender.logFile != nil {
		if err := appender.logFile.Close(); err != nil {
			fmt.Printf("[logger appender] close logfile err: %s\n", err)
		}
		appender.logFile = nil
	}
}

// 拦截panic
func onError(txt string) {
	if r := recover(); r != nil {
		fmt.Println(time.Now().Format(_timeFormat), "[ERROR] -", "Got a runtime error %s. %s\n%s", txt, r, string(debug.Stack()))
	}
}
