package taskx

import (
	"errors"
	"strings"
	"sync"

	"github.com/caiflower/common-tools/taskx/executor"
)

var (
	ErrNonRetryable = errors.New("non-retryable error")
)

// _providerRegistry 全局执行器注册表（集群框架要求，receiver 从数据库读取时查找）
// taskName -> subTaskName -> ExecutorProvider
var (
	_providerRegistry = struct {
		sync.RWMutex
		providers map[string]map[string]executor.ExecutorProvider
	}{providers: make(map[string]map[string]executor.ExecutorProvider)}
)

// registerProvider 注册子任务执行器（全局）
func registerProvider(taskName, subTaskName string, p executor.ExecutorProvider) {
	_providerRegistry.Lock()
	defer _providerRegistry.Unlock()
	if _providerRegistry.providers[taskName] == nil {
		_providerRegistry.providers[taskName] = make(map[string]executor.ExecutorProvider)
	}
	_providerRegistry.providers[taskName][subTaskName] = p
}

// registerProviders 批量注册执行器（全局）
func registerProviders(taskName string, providers map[string]executor.ExecutorProvider) {
	_providerRegistry.Lock()
	defer _providerRegistry.Unlock()
	if _providerRegistry.providers[taskName] == nil {
		_providerRegistry.providers[taskName] = make(map[string]executor.ExecutorProvider)
	}
	for name, p := range providers {
		_providerRegistry.providers[taskName][name] = p
	}
}

// getProvider 查找子任务执行器（全局）
func getProvider(taskName, subTaskName string) executor.ExecutorProvider {
	_providerRegistry.RLock()
	defer _providerRegistry.RUnlock()
	if providers, ok := _providerRegistry.providers[taskName]; ok {
		return providers[subTaskName]
	}
	return nil
}

// registerRollbackProvider 注册回滚执行器（全局）
var (
	_rollbackRegistry = struct {
		sync.RWMutex
		providers map[string]executor.ExecutorProvider // key: taskName/subTaskName
	}{providers: make(map[string]executor.ExecutorProvider)}
)

func registerRollbackProvider(taskName, subTaskName string, p executor.ExecutorProvider) {
	_rollbackRegistry.Lock()
	defer _rollbackRegistry.Unlock()
	_rollbackRegistry.providers[taskName+"/"+subTaskName] = p
}

func getRollbackProvider(taskName, subTaskName string) executor.ExecutorProvider {
	_rollbackRegistry.RLock()
	defer _rollbackRegistry.RUnlock()
	return _rollbackRegistry.providers[taskName+"/"+subTaskName]
}

// 全局 TaskExecutor 注册表（集群框架：receiver 从数据库读取时需要）
var (
	_taskExecutorRegistry = struct {
		sync.RWMutex
		executors map[string]TaskExecutor
	}{executors: make(map[string]TaskExecutor)}
)

// _customRollbackRegistry 全局自定义回滚函数注册表（集群框架：dispatcher 从 DB 重建 Task 时恢复）
// taskName -> custom rollback function
var (
	_customRollbackRegistry = struct {
		sync.RWMutex
		funcs map[string]func(completed []string, failed string) []string
	}{funcs: make(map[string]func(completed []string, failed string) []string)}
)

func registerCustomRollback(taskName string, fn func(completed []string, failed string) []string) {
	_customRollbackRegistry.Lock()
	defer _customRollbackRegistry.Unlock()
	_customRollbackRegistry.funcs[taskName] = fn
}

func getCustomRollback(taskName string) func(completed []string, failed string) []string {
	_customRollbackRegistry.RLock()
	defer _customRollbackRegistry.RUnlock()
	return _customRollbackRegistry.funcs[taskName]
}

func registerTaskExecutor(e TaskExecutor) {
	_taskExecutorRegistry.Lock()
	defer _taskExecutorRegistry.Unlock()
	_taskExecutorRegistry.executors[e.Name()] = e
}

func getTaskExecutor(taskName string) TaskExecutor {
	_taskExecutorRegistry.RLock()
	defer _taskExecutorRegistry.RUnlock()
	return _taskExecutorRegistry.executors[taskName]
}

// ClearProviders 清理指定 taskName 的全局执行器注册表
func ClearProviders(taskName string) {
	_providerRegistry.Lock()
	defer _providerRegistry.Unlock()
	delete(_providerRegistry.providers, taskName)

	_rollbackRegistry.Lock()
	defer _rollbackRegistry.Unlock()
	for key := range _rollbackRegistry.providers {
		if strings.HasPrefix(key, taskName+"/") {
			delete(_rollbackRegistry.providers, key)
		}
	}

	_branchRegistry.Lock()
	defer _branchRegistry.Unlock()
	delete(_branchRegistry.branches, taskName)

	_customRollbackRegistry.Lock()
	defer _customRollbackRegistry.Unlock()
	delete(_customRollbackRegistry.funcs, taskName)
}

// ClearAllProviders 清理所有全局执行器注册表
func ClearAllProviders() {
	_providerRegistry.Lock()
	defer _providerRegistry.Unlock()
	_providerRegistry.providers = make(map[string]map[string]executor.ExecutorProvider)

	_rollbackRegistry.Lock()
	defer _rollbackRegistry.Unlock()
	_rollbackRegistry.providers = make(map[string]executor.ExecutorProvider)

	_branchRegistry.Lock()
	defer _branchRegistry.Unlock()
	_branchRegistry.branches = make(map[string]map[string][]*Branch)

	_customRollbackRegistry.Lock()
	defer _customRollbackRegistry.Unlock()
	_customRollbackRegistry.funcs = make(map[string]func(completed []string, failed string) []string)
}

// ClearTaskExecutors 清理指定 taskName 的全局 TaskExecutor 注册表
func ClearTaskExecutors(taskName string) {
	_taskExecutorRegistry.Lock()
	defer _taskExecutorRegistry.Unlock()
	delete(_taskExecutorRegistry.executors, taskName)
}

// ClearAllTaskExecutors 清理所有全局 TaskExecutor 注册表
func ClearAllTaskExecutors() {
	_taskExecutorRegistry.Lock()
	defer _taskExecutorRegistry.Unlock()
	_taskExecutorRegistry.executors = make(map[string]TaskExecutor)
}

// _branchRegistry 全局分支注册表（集群框架：dispatcher 从 DB 重建 Task 时恢复分支信息）
// taskName -> nodeKey -> []*Branch
var (
	_branchRegistry = struct {
		sync.RWMutex
		branches map[string]map[string][]*Branch
	}{branches: make(map[string]map[string][]*Branch)}
)

func registerBranch(taskName, nodeKey string, branch *Branch) {
	_branchRegistry.Lock()
	defer _branchRegistry.Unlock()
	if _branchRegistry.branches[taskName] == nil {
		_branchRegistry.branches[taskName] = make(map[string][]*Branch)
	}
	_branchRegistry.branches[taskName][nodeKey] = append(_branchRegistry.branches[taskName][nodeKey], branch)
}

func getRegisteredBranches(taskName string) map[string][]*Branch {
	_branchRegistry.RLock()
	defer _branchRegistry.RUnlock()
	return _branchRegistry.branches[taskName]
}

// _processorRegistry 全局处理器注册表（集群框架：receiver 从数据库恢复时查找 preProcessor/postProcessor）
var (
	_preProcessorRegistry = struct {
		sync.RWMutex
		processors map[string]map[string]Processor // taskName -> subTaskName -> Processor
	}{processors: make(map[string]map[string]Processor)}

	_postProcessorRegistry = struct {
		sync.RWMutex
		processors map[string]map[string]Processor // taskName -> subTaskName -> Processor
	}{processors: make(map[string]map[string]Processor)}
)

func registerPreProcessor(taskName, subTaskName string, p Processor) {
	_preProcessorRegistry.Lock()
	defer _preProcessorRegistry.Unlock()
	if _preProcessorRegistry.processors[taskName] == nil {
		_preProcessorRegistry.processors[taskName] = make(map[string]Processor)
	}
	_preProcessorRegistry.processors[taskName][subTaskName] = p
}

func getPreProcessor(taskName, subTaskName string) Processor {
	_preProcessorRegistry.RLock()
	defer _preProcessorRegistry.RUnlock()
	if processors, ok := _preProcessorRegistry.processors[taskName]; ok {
		return processors[subTaskName]
	}
	return nil
}

func registerPostProcessor(taskName, subTaskName string, p Processor) {
	_postProcessorRegistry.Lock()
	defer _postProcessorRegistry.Unlock()
	if _postProcessorRegistry.processors[taskName] == nil {
		_postProcessorRegistry.processors[taskName] = make(map[string]Processor)
	}
	_postProcessorRegistry.processors[taskName][subTaskName] = p
}

func getPostProcessor(taskName, subTaskName string) Processor {
	_postProcessorRegistry.RLock()
	defer _postProcessorRegistry.RUnlock()
	if processors, ok := _postProcessorRegistry.processors[taskName]; ok {
		return processors[subTaskName]
	}
	return nil
}
