/*
 * Copyright 2024 caiflower Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

 package logger

import (
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"sort"
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
	rollingCutLog()
	loadLoggerFiles()
	compressLog()
	cleanLog()
	close()
}

const (
	gz = ".gz"
)

type logAppender struct {
	timeFormat        string
	isConsole         bool
	enableTrace       bool
	enableCompress    bool
	enableCleanBackup bool
	dir               string
	fileName          string
	rollingPolicy     string
	maxTime           int64
	maxSize           int64
	backupMaxCount    int
	backupMaxDiskSize int64
	enableColor       bool

	bufPool             sync.Pool
	log                 *log.Logger
	writeLock           sync.Locker
	compressLock        sync.Locker
	logFile             *os.File
	filesize            int64
	lastTime            time.Time
	backLoggerFiles     []string
	backLoggerFilesSize int64
}

func newLogAppender(timeFormat, path, fileName, rollingPolicy, maxTime, maxSize, backupMaxSize string, backupMaxCount int, enableTrace, enableCompress, enableCleanBackup, enableColor bool) Appender {
	appender := &logAppender{
		timeFormat: timeFormat,
		bufPool: sync.Pool{
			New: func() interface{} {
				return new(strings.Builder)
			}},
		dir:               path,
		fileName:          fileName,
		rollingPolicy:     rollingPolicy,
		enableTrace:       enableTrace,
		enableCompress:    enableCompress,
		enableCleanBackup: enableCleanBackup,
		writeLock:         syncx.NewSpinLock(),
		compressLock:      syncx.NewSpinLock(),
		log:               new(log.Logger),
		maxSize:           getMaxSize(maxSize),
		maxTime:           getMaxTime(maxTime),
		backupMaxCount:    backupMaxCount,
		backupMaxDiskSize: getMaxSize(backupMaxSize),
		enableColor:       enableColor,
	}

	if appender.maxSize == 0 && appender.maxTime == 0 {
		appender.rollingPolicy = ""
	}

	if appender.dir == "" {
		appender.isConsole = true
		appender.log.SetOutput(os.Stdout)
	} else {
		// 创建目录
		err := tools.Mkdir(appender.dir, 0755)
		if err != nil {
			panic(fmt.Sprintf("[logger appender] mkdir err: %s\n", err))
		}

		// 创建文件流
		appender.createFilestream()
		appender.loadLoggerFiles()
		// 启动先压缩一遍文件
		go func() {
			appender.compressLog()
			appender.cleanLog()
		}()
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
		num *= 1024 * 1024 * 1024
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
		appender.logFile = nil
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

	appender.rollingCutLog()

	timeFormat := data.timestamp.Format(appender.timeFormat)
	if appender.enableColor {
		data.level = getLevelColor(data.level)
	}
	buf := appender.bufPool.Get().(*strings.Builder)
	buf.Reset()
	buf.WriteString(timeFormat)
	buf.WriteString(" [")
	buf.WriteString(data.level)
	buf.WriteString("] ")
	if appender.enableTrace && data.traceID != "" {
		if appender.enableColor {
			data.traceID = fmt.Sprintf("\033[1;35m%s\033[0m", data.traceID)
		}
		buf.WriteString("[")
		buf.WriteString(data.traceID)
		buf.WriteString("] ")
	}
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

func (appender *logAppender) rollingCutLog() {
	if appender.needCutLog() {
		appender.writeLock.Lock()
		defer appender.writeLock.Unlock()
		if appender.needCutLog() {
			appender.cutLog()
		}
	}
}

func (appender *logAppender) cutLog() {
	if appender.logFile != nil {
		if err := appender.logFile.Close(); err != nil {
			fmt.Printf("[logger appender] close logfile err: %s\n", err)
		}
		appender.logFile = nil
	}

	appender.compressLock.Lock()
	defer appender.compressLock.Unlock()
	targetFileName := appender.fileName + "-" + time.Now().Format("20060102150405")
	i := 0
	for {
		if (i == 0 && tools.StringSliceContains(appender.backLoggerFiles, targetFileName)) ||
			(i == 0 && tools.StringSliceContains(appender.backLoggerFiles, targetFileName+gz)) ||
			(i != 0 && tools.StringSliceContains(appender.backLoggerFiles, targetFileName+"-"+strconv.Itoa(i))) ||
			(i != 0 && tools.StringSliceContains(appender.backLoggerFiles, targetFileName+"-"+strconv.Itoa(i)+gz)) {
			i++
		} else {
			if i != 0 {
				targetFileName = targetFileName + "-" + strconv.Itoa(i)
			}
			break
		}
	}

	targetFilePath := appender.dir + "/" + targetFileName
	if err := os.Rename(appender.dir+"/"+appender.fileName, targetFilePath); err != nil {
		fmt.Printf("[logger appender] rename logfile err: %s\n", err)
	}
	// 切换文件
	appender.createFilestream()
	appender.loadLoggerFiles()

	// 压缩日志
	go func() {
		appender.compressLog()
		appender.cleanLog()
	}()
}

func (appender *logAppender) loadLoggerFiles() {
	if dir, err := os.ReadDir(appender.dir); err != nil {
		panic(fmt.Sprintf("[logger appender] loadLoggerFiles err: %s\n", err))
	} else {
		appender.backLoggerFiles = []string{}
		appender.backLoggerFilesSize = 0
		for _, v := range dir {
			if !v.IsDir() {
				if name := v.Name(); strings.HasPrefix(name, appender.fileName) && name != appender.fileName {
					appender.backLoggerFiles = append(appender.backLoggerFiles, name)
					info, _ := v.Info()
					appender.backLoggerFilesSize += info.Size()
				}
			}
		}
		sort.Slice(appender.backLoggerFiles, func(i, j int) bool {
			a := strings.Split(strings.Replace(strings.Replace(appender.backLoggerFiles[i], appender.fileName+"-", "", 1), gz, "", 1), "-")
			b := strings.Split(strings.Replace(strings.Replace(appender.backLoggerFiles[j], appender.fileName+"-", "", 1), gz, "", 1), "-")
			if a[0] == b[0] {
				if len(a) == len(b) {
					if len(a) == 2 {
						num1, _ := strconv.Atoi(a[1])
						num2, _ := strconv.Atoi(b[1])
						return num1 < num2
					} else {
						return true
					}
				} else {
					return len(a) < len(b)
				}
			} else {
				return a[0] < b[0]
			}
		})
	}

}

func (appender *logAppender) compressLog() {
	defer onError("logger compress")

	if appender.enableCompress {
		appender.compressLock.Lock()
		defer appender.compressLock.Unlock()

		for _, v := range appender.backLoggerFiles {
			if !strings.HasSuffix(v, gz) {
				filepath := appender.dir + "/" + v
				if tools.FileExist(filepath) {
					fmt.Println(time.Now().Format(_timeFormat), "[LOG] compress log ->", filepath)
					if tools.FileExist(filepath + gz) {
						size, _ := tools.FileSize(filepath + gz)
						appender.backLoggerFilesSize -= size
						if err := os.Remove(filepath + gz); err != nil {
							fmt.Printf("[logger compress] remove log file err: %s\n", err)
						}
					}

					appender.compressFile(v)
				}
			}
		}

		appender.loadLoggerFiles()
	}
}

func (appender *logAppender) compressFile(fName string) {
	filepath := fmt.Sprint(appender.dir, "/", fName)

	// 源文件
	fromFile, _ := os.Open(filepath)
	defer fromFile.Close()

	// 目标文件
	targetFile, _ := os.Create(filepath + gz)
	defer targetFile.Close()

	// gzip
	gzipWriter := gzip.NewWriter(targetFile)
	defer gzipWriter.Close()

	io.Copy(gzipWriter, fromFile)
	os.Remove(filepath)
}

func (appender *logAppender) cleanLog() {
	if appender.enableCleanBackup {
		appender.compressLock.Lock()
		defer appender.compressLock.Unlock()

		i := 0
		var removeSize int64
		for len(appender.backLoggerFiles)-i > appender.backupMaxCount || appender.backLoggerFilesSize-removeSize > appender.backupMaxDiskSize {
			fileName := appender.backLoggerFiles[i]

			filepath := appender.dir + "/" + fileName
			fmt.Println(time.Now().Format(_timeFormat), "[LOG] clean log ->", filepath)
			size, _ := tools.FileSize(filepath)
			removeSize += size
			os.Remove(filepath)
			i++
		}

		appender.loadLoggerFiles()
	}
}

func (appender *logAppender) needCutLog() bool {
	if appender.dir == "" {
		return false
	}
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
		if err := appender.logFile.Sync(); err != nil {
			fmt.Printf("[logger close] sync log file err: %s\n", err)
		}

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
