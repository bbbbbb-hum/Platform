package logger

import (
	"AgentEarth_AgentPlatform/src/helpers"
	"AgentEarth_AgentPlatform/src/helpers/app"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger 全局 Logger 对象
var Logger *zap.Logger

// dailyRotateWriter 按日期自动切换日志文件的 WriteSyncer
type dailyRotateWriter struct {
	baseFilename  string // 基础文件名，如 storage/logs/logs.log
	maxSize       int
	maxBackups    int
	maxAge        int
	compress      bool
	currentDate   string // 当前日期，格式：2006-01-02
	currentLogger *lumberjack.Logger
	mu            sync.Mutex
}

// newDailyRotateWriter 创建按日期自动切换的日志写入器
func newDailyRotateWriter(baseFilename string, maxSize, maxBackups, maxAge int, compress bool) *dailyRotateWriter {
	writer := &dailyRotateWriter{
		baseFilename: baseFilename,
		maxSize:      maxSize,
		maxBackups:   maxBackups,
		maxAge:       maxAge,
		compress:     compress,
	}
	// 初始化当前日期的日志文件
	writer.rotateIfNeeded()
	return writer
}

// rotateIfNeeded 检查日期并切换日志文件（如果需要）
func (w *dailyRotateWriter) rotateIfNeeded() {
	now := helpers.TimeNowInTimezone()
	currentDate := now.Format("2006-01-02")

	w.mu.Lock()
	defer w.mu.Unlock()

	// 如果日期没有变化，不需要切换
	if currentDate == w.currentDate && w.currentLogger != nil {
		return
	}

	// 日期变化，创建新的日志文件
	w.currentDate = currentDate
	logname := currentDate + ".log"

	// 从基础文件名中提取目录
	dir := filepath.Dir(w.baseFilename)
	newFilename := filepath.Join(dir, logname)

	// 创建新的 lumberjack.Logger
	w.currentLogger = &lumberjack.Logger{
		Filename:   newFilename,
		MaxSize:    w.maxSize,
		MaxBackups: w.maxBackups,
		MaxAge:     w.maxAge,
		Compress:   w.compress,
	}
}

// Write 实现 io.Writer 接口
func (w *dailyRotateWriter) Write(p []byte) (n int, err error) {
	// 每次写入前检查日期
	w.rotateIfNeeded()

	w.mu.Lock()
	logger := w.currentLogger
	w.mu.Unlock()

	return logger.Write(p)
}

// Sync 实现 zapcore.WriteSyncer 接口
func (w *dailyRotateWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.currentLogger != nil {
		// lumberjack.Logger 没有 Sync 方法，返回 nil
		return nil
	}
	return nil
}

// InitLogger 日志初始化
func InitLogger(filename string, maxSize, maxBackup, maxAge int, compress bool, logType string, level string, isLocal bool) {
	// 获取日志写入介质
	writeSyncer := getLogWriter(filename, maxSize, maxBackup, maxAge, compress, logType, isLocal)
	// 设置日志等级，具体请见 config/log.go 文件
	logLevel := new(zapcore.Level)
	if err := logLevel.UnmarshalText([]byte(level)); err != nil {
		fmt.Printf("日志初始化错误，日志级别设置有误: %v，将使用默认级别 info\n", err)
		// 解析失败时使用 info 作为默认级别
		*logLevel = zapcore.InfoLevel
	}
	fmt.Printf("日志级别: %v\n", logLevel)
	// 初始化 core
	core := zapcore.NewCore(getEncoder(), writeSyncer, logLevel)
	// 初始化 Logger
	Logger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1), zap.AddStacktrace(zap.ErrorLevel))
	// 将自定义的 logger 替换为全局的 logger
	// zap.L().Fatal() 调用时，就会使用我们自定的 Logger
	zap.ReplaceGlobals(Logger)
}

// getLogWriter 日志记录介质。Gohub 中使用了两种介质，os.Stdout 和文件
func getLogWriter(filename string, maxSize, maxBackup, maxAge int, compress bool, logType string, isLocal bool) zapcore.WriteSyncer {
	var fileWriter zapcore.WriteSyncer

	// 如果配置了按照日期记录日志文件
	if logType == "daily" {
		// 使用按日期自动切换的写入器
		dailyWriter := newDailyRotateWriter(filename, maxSize, maxBackup, maxAge, compress)
		fileWriter = dailyWriter
	} else {
		// 单文件模式，使用 lumberjack 滚动日志
		lumberJackLogger := &lumberjack.Logger{
			Filename:   filename,
			MaxSize:    maxSize,
			MaxBackups: maxBackup,
			MaxAge:     maxAge,
			Compress:   compress,
		}
		fileWriter = zapcore.AddSync(lumberJackLogger)
	}

	// 配置输出介质
	if isLocal {
		// 本地开发终端打印和记录文件
		return zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), fileWriter)
	} else {
		// 生产环境只记录文件
		return fileWriter
	}
}

// getEncoder 设置日志存储格式
func getEncoder() zapcore.Encoder {
	// 日志格式规则
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller", // 代码调用，如 paginator/paginator.go:148
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,      // 每行日志的结尾添加 "\n"
		EncodeLevel:    zapcore.CapitalLevelEncoder,    // 日志级别名称大写，如 ERROR、INFO
		EncodeTime:     customTimeEncoder,              // 时间格式，我们自定义为 2006-01-02 15:04:05
		EncodeDuration: zapcore.SecondsDurationEncoder, // 执行时间，以秒为单位
		EncodeCaller:   zapcore.ShortCallerEncoder,     // Caller 短格式，如：types/converter.go:17，长格式为绝对路径
	}
	// 本地环境配置
	if app.IsLocal() {
		// 终端输出的关键词高亮
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		// 本地设置内置的 Console 解码器（支持 stacktrace 换行）
		return zapcore.NewConsoleEncoder(encoderConfig)
	}
	// 线上环境使用 JSON 编码器
	return zapcore.NewJSONEncoder(encoderConfig)
}

// customTimeEncoder 自定义友好的时间格式
func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	loc, _ := time.LoadLocation("Asia/Shanghai") // 东八区
	t = t.In(loc)                                // 转换为指定时区的时间
	enc.AppendString(t.Format("2006-01-02 15:04:05"))
}

// Dump 调试专用，不会中断程序，会在终端打印出 warning 消息。
func Dump(value interface{}, msg ...string) {
	valueString := jsonString(value)
	// 判断第二个参数是否传参 msg
	if len(msg) > 0 {
		Logger.Warn("Dump", zap.String(msg[0], valueString))
	} else {
		Logger.Warn("Dump", zap.String("data", valueString))
	}
}

func jsonString(value interface{}) string {
	b, err := json.Marshal(value)
	if err != nil {
		Logger.Error("Logger", zap.String("JSON marshal error", err.Error()))
	}
	return string(b)
}

// LogIf 当 err != nil 时记录 error 等级的日志
func LogIf(err error) {
	if err != nil {
		Logger.Error("Error Occurred:", zap.Error(err))
	}
}

// LogWarnIf 当 err != nil 时记录 warning 等级的日志
func LogWarnIf(err error) {
	if err != nil {
		Logger.Warn("Error Occurred:", zap.Error(err))
	}
}

// LogInfoIf 当 err != nil 时记录 info 等级的日志
func LogInfoIf(err error) {
	if err != nil {
		Logger.Info("Error Occurred:", zap.Error(err))
	}
}

// Debug 调试日志，详尽的程序日志
func Debug(moduleName string, fields ...zap.Field) {
	Logger.Debug(moduleName, fields...)
}

// Info 告知类日志
func Info(moduleName string, fields ...zap.Field) {
	Logger.Info(moduleName, fields...)
}

// Warn 警告类
func Warn(moduleName string, fields ...zap.Field) {
	Logger.Warn(moduleName, fields...)
}

// Error 错误时记录，不应该中断程序，查看日志时重点关注
func Error(moduleName string, fields ...zap.Field) {
	Logger.Error(moduleName, fields...)
}

// Fatal 级别同 Error(), 写完 log 后调用 os.Exit(1) 退出程序 logger.DebugString("SMS", "短信内容", string(result.RawResponse))
func Fatal(moduleName string, fields ...zap.Field) {
	Logger.Fatal(moduleName, fields...)
}

// DebugString 记录一条字符串类型的 debug 日志，调用示例：
func DebugString(moduleName, name, msg string) {
	Logger.Debug(moduleName, zap.String(name, msg))
}

func InfoString(moduleName, name, msg string) {
	Logger.Info(moduleName, zap.String(name, msg))
}

func WarnString(moduleName, name, msg string) {
	Logger.Warn(moduleName, zap.String(name, msg))
}

func ErrorString(moduleName, name, msg string) {
	Logger.Error(moduleName, zap.String(name, msg))
}

func FatalString(moduleName, name, msg string) {
	Logger.Fatal(moduleName, zap.String(name, msg))
}

// DebugJSON 记录对象类型的 debug 日志，使用 json.Marshal 进行编码。调用示例： logger.DebugJSON("Auth", "读取登录用户", auth.CurrentUser())
func DebugJSON(moduleName, name string, value interface{}) {
	Logger.Debug(moduleName, zap.String(name, jsonString(value)))
}

func InfoJSON(moduleName, name string, value interface{}) {
	Logger.Info(moduleName, zap.String(name, jsonString(value)))
}

func WarnJSON(moduleName, name string, value interface{}) {
	Logger.Warn(moduleName, zap.String(name, jsonString(value)))
}

func ErrorJSON(moduleName, name string, value interface{}) {
	Logger.Error(moduleName, zap.String(name, jsonString(value)))
}

func FatalJSON(moduleName, name string, value interface{}) {
	Logger.Fatal(moduleName, zap.String(name, jsonString(value)))
}
