package web

type Config struct {
	Name                  string     `yaml:"name" default:"default"`
	Port                  uint       `yaml:"port" default:"8080"`
	ReadTimeout           uint       `yaml:"readTimeout" default:"20"`
	WriteTimeout          uint       `yaml:"writeTimeout" default:"35"`
	HandleTimeout         *uint      `yaml:"handleTimeout" default:"60"` // 请求总处理超时时间
	RootPath              string     `yaml:"rootPath"`                   // 可以为空
	HeaderTraceID         string     `yaml:"headerTraceID" default:"X-Request-Id"`
	ControllerRootPkgName string     `yaml:"controllerRootPkgName" default:"controller"`
	WebLimiter            WebLimiter `yaml:"webLimiter"`
	EnablePprof           bool       `yaml:"enablePprof"`
}
