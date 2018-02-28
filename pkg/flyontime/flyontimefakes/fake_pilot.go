// Code generated by counterfeiter. DO NOT EDIT.
package flyontimefakes

import (
	"context"
	"sync"

	"github.com/Bo0mer/flyontime/pkg/flyontime"
	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse"
)

type FakePilot struct {
	URLStub        func() string
	uRLMutex       sync.RWMutex
	uRLArgsForCall []struct{}
	uRLReturns     struct {
		result1 string
	}
	uRLReturnsOnCall map[int]struct {
		result1 string
	}
	PausePipelineStub        func(pipeline string) (bool, error)
	pausePipelineMutex       sync.RWMutex
	pausePipelineArgsForCall []struct {
		pipeline string
	}
	pausePipelineReturns struct {
		result1 bool
		result2 error
	}
	pausePipelineReturnsOnCall map[int]struct {
		result1 bool
		result2 error
	}
	UnpausePipelineStub        func(pipeline string) (bool, error)
	unpausePipelineMutex       sync.RWMutex
	unpausePipelineArgsForCall []struct {
		pipeline string
	}
	unpausePipelineReturns struct {
		result1 bool
		result2 error
	}
	unpausePipelineReturnsOnCall map[int]struct {
		result1 bool
		result2 error
	}
	PauseJobStub        func(pipeline, job string) (bool, error)
	pauseJobMutex       sync.RWMutex
	pauseJobArgsForCall []struct {
		pipeline string
		job      string
	}
	pauseJobReturns struct {
		result1 bool
		result2 error
	}
	pauseJobReturnsOnCall map[int]struct {
		result1 bool
		result2 error
	}
	UnpauseJobStub        func(pipeline, job string) (bool, error)
	unpauseJobMutex       sync.RWMutex
	unpauseJobArgsForCall []struct {
		pipeline string
		job      string
	}
	unpauseJobReturns struct {
		result1 bool
		result2 error
	}
	unpauseJobReturnsOnCall map[int]struct {
		result1 bool
		result2 error
	}
	CreateJobBuildStub        func(pipeline, job string) (atc.Build, error)
	createJobBuildMutex       sync.RWMutex
	createJobBuildArgsForCall []struct {
		pipeline string
		job      string
	}
	createJobBuildReturns struct {
		result1 atc.Build
		result2 error
	}
	createJobBuildReturnsOnCall map[int]struct {
		result1 atc.Build
		result2 error
	}
	BuildEventsStub        func(job string) (concourse.Events, error)
	buildEventsMutex       sync.RWMutex
	buildEventsArgsForCall []struct {
		job string
	}
	buildEventsReturns struct {
		result1 concourse.Events
		result2 error
	}
	buildEventsReturnsOnCall map[int]struct {
		result1 concourse.Events
		result2 error
	}
	FinishedBuildsStub        func(ctx context.Context) <-chan atc.Build
	finishedBuildsMutex       sync.RWMutex
	finishedBuildsArgsForCall []struct {
		ctx context.Context
	}
	finishedBuildsReturns struct {
		result1 <-chan atc.Build
	}
	finishedBuildsReturnsOnCall map[int]struct {
		result1 <-chan atc.Build
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakePilot) URL() string {
	fake.uRLMutex.Lock()
	ret, specificReturn := fake.uRLReturnsOnCall[len(fake.uRLArgsForCall)]
	fake.uRLArgsForCall = append(fake.uRLArgsForCall, struct{}{})
	fake.recordInvocation("URL", []interface{}{})
	fake.uRLMutex.Unlock()
	if fake.URLStub != nil {
		return fake.URLStub()
	}
	if specificReturn {
		return ret.result1
	}
	return fake.uRLReturns.result1
}

func (fake *FakePilot) URLCallCount() int {
	fake.uRLMutex.RLock()
	defer fake.uRLMutex.RUnlock()
	return len(fake.uRLArgsForCall)
}

func (fake *FakePilot) URLReturns(result1 string) {
	fake.URLStub = nil
	fake.uRLReturns = struct {
		result1 string
	}{result1}
}

func (fake *FakePilot) URLReturnsOnCall(i int, result1 string) {
	fake.URLStub = nil
	if fake.uRLReturnsOnCall == nil {
		fake.uRLReturnsOnCall = make(map[int]struct {
			result1 string
		})
	}
	fake.uRLReturnsOnCall[i] = struct {
		result1 string
	}{result1}
}

func (fake *FakePilot) PausePipeline(pipeline string) (bool, error) {
	fake.pausePipelineMutex.Lock()
	ret, specificReturn := fake.pausePipelineReturnsOnCall[len(fake.pausePipelineArgsForCall)]
	fake.pausePipelineArgsForCall = append(fake.pausePipelineArgsForCall, struct {
		pipeline string
	}{pipeline})
	fake.recordInvocation("PausePipeline", []interface{}{pipeline})
	fake.pausePipelineMutex.Unlock()
	if fake.PausePipelineStub != nil {
		return fake.PausePipelineStub(pipeline)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.pausePipelineReturns.result1, fake.pausePipelineReturns.result2
}

func (fake *FakePilot) PausePipelineCallCount() int {
	fake.pausePipelineMutex.RLock()
	defer fake.pausePipelineMutex.RUnlock()
	return len(fake.pausePipelineArgsForCall)
}

func (fake *FakePilot) PausePipelineArgsForCall(i int) string {
	fake.pausePipelineMutex.RLock()
	defer fake.pausePipelineMutex.RUnlock()
	return fake.pausePipelineArgsForCall[i].pipeline
}

func (fake *FakePilot) PausePipelineReturns(result1 bool, result2 error) {
	fake.PausePipelineStub = nil
	fake.pausePipelineReturns = struct {
		result1 bool
		result2 error
	}{result1, result2}
}

func (fake *FakePilot) PausePipelineReturnsOnCall(i int, result1 bool, result2 error) {
	fake.PausePipelineStub = nil
	if fake.pausePipelineReturnsOnCall == nil {
		fake.pausePipelineReturnsOnCall = make(map[int]struct {
			result1 bool
			result2 error
		})
	}
	fake.pausePipelineReturnsOnCall[i] = struct {
		result1 bool
		result2 error
	}{result1, result2}
}

func (fake *FakePilot) UnpausePipeline(pipeline string) (bool, error) {
	fake.unpausePipelineMutex.Lock()
	ret, specificReturn := fake.unpausePipelineReturnsOnCall[len(fake.unpausePipelineArgsForCall)]
	fake.unpausePipelineArgsForCall = append(fake.unpausePipelineArgsForCall, struct {
		pipeline string
	}{pipeline})
	fake.recordInvocation("UnpausePipeline", []interface{}{pipeline})
	fake.unpausePipelineMutex.Unlock()
	if fake.UnpausePipelineStub != nil {
		return fake.UnpausePipelineStub(pipeline)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.unpausePipelineReturns.result1, fake.unpausePipelineReturns.result2
}

func (fake *FakePilot) UnpausePipelineCallCount() int {
	fake.unpausePipelineMutex.RLock()
	defer fake.unpausePipelineMutex.RUnlock()
	return len(fake.unpausePipelineArgsForCall)
}

func (fake *FakePilot) UnpausePipelineArgsForCall(i int) string {
	fake.unpausePipelineMutex.RLock()
	defer fake.unpausePipelineMutex.RUnlock()
	return fake.unpausePipelineArgsForCall[i].pipeline
}

func (fake *FakePilot) UnpausePipelineReturns(result1 bool, result2 error) {
	fake.UnpausePipelineStub = nil
	fake.unpausePipelineReturns = struct {
		result1 bool
		result2 error
	}{result1, result2}
}

func (fake *FakePilot) UnpausePipelineReturnsOnCall(i int, result1 bool, result2 error) {
	fake.UnpausePipelineStub = nil
	if fake.unpausePipelineReturnsOnCall == nil {
		fake.unpausePipelineReturnsOnCall = make(map[int]struct {
			result1 bool
			result2 error
		})
	}
	fake.unpausePipelineReturnsOnCall[i] = struct {
		result1 bool
		result2 error
	}{result1, result2}
}

func (fake *FakePilot) PauseJob(pipeline string, job string) (bool, error) {
	fake.pauseJobMutex.Lock()
	ret, specificReturn := fake.pauseJobReturnsOnCall[len(fake.pauseJobArgsForCall)]
	fake.pauseJobArgsForCall = append(fake.pauseJobArgsForCall, struct {
		pipeline string
		job      string
	}{pipeline, job})
	fake.recordInvocation("PauseJob", []interface{}{pipeline, job})
	fake.pauseJobMutex.Unlock()
	if fake.PauseJobStub != nil {
		return fake.PauseJobStub(pipeline, job)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.pauseJobReturns.result1, fake.pauseJobReturns.result2
}

func (fake *FakePilot) PauseJobCallCount() int {
	fake.pauseJobMutex.RLock()
	defer fake.pauseJobMutex.RUnlock()
	return len(fake.pauseJobArgsForCall)
}

func (fake *FakePilot) PauseJobArgsForCall(i int) (string, string) {
	fake.pauseJobMutex.RLock()
	defer fake.pauseJobMutex.RUnlock()
	return fake.pauseJobArgsForCall[i].pipeline, fake.pauseJobArgsForCall[i].job
}

func (fake *FakePilot) PauseJobReturns(result1 bool, result2 error) {
	fake.PauseJobStub = nil
	fake.pauseJobReturns = struct {
		result1 bool
		result2 error
	}{result1, result2}
}

func (fake *FakePilot) PauseJobReturnsOnCall(i int, result1 bool, result2 error) {
	fake.PauseJobStub = nil
	if fake.pauseJobReturnsOnCall == nil {
		fake.pauseJobReturnsOnCall = make(map[int]struct {
			result1 bool
			result2 error
		})
	}
	fake.pauseJobReturnsOnCall[i] = struct {
		result1 bool
		result2 error
	}{result1, result2}
}

func (fake *FakePilot) UnpauseJob(pipeline string, job string) (bool, error) {
	fake.unpauseJobMutex.Lock()
	ret, specificReturn := fake.unpauseJobReturnsOnCall[len(fake.unpauseJobArgsForCall)]
	fake.unpauseJobArgsForCall = append(fake.unpauseJobArgsForCall, struct {
		pipeline string
		job      string
	}{pipeline, job})
	fake.recordInvocation("UnpauseJob", []interface{}{pipeline, job})
	fake.unpauseJobMutex.Unlock()
	if fake.UnpauseJobStub != nil {
		return fake.UnpauseJobStub(pipeline, job)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.unpauseJobReturns.result1, fake.unpauseJobReturns.result2
}

func (fake *FakePilot) UnpauseJobCallCount() int {
	fake.unpauseJobMutex.RLock()
	defer fake.unpauseJobMutex.RUnlock()
	return len(fake.unpauseJobArgsForCall)
}

func (fake *FakePilot) UnpauseJobArgsForCall(i int) (string, string) {
	fake.unpauseJobMutex.RLock()
	defer fake.unpauseJobMutex.RUnlock()
	return fake.unpauseJobArgsForCall[i].pipeline, fake.unpauseJobArgsForCall[i].job
}

func (fake *FakePilot) UnpauseJobReturns(result1 bool, result2 error) {
	fake.UnpauseJobStub = nil
	fake.unpauseJobReturns = struct {
		result1 bool
		result2 error
	}{result1, result2}
}

func (fake *FakePilot) UnpauseJobReturnsOnCall(i int, result1 bool, result2 error) {
	fake.UnpauseJobStub = nil
	if fake.unpauseJobReturnsOnCall == nil {
		fake.unpauseJobReturnsOnCall = make(map[int]struct {
			result1 bool
			result2 error
		})
	}
	fake.unpauseJobReturnsOnCall[i] = struct {
		result1 bool
		result2 error
	}{result1, result2}
}

func (fake *FakePilot) CreateJobBuild(pipeline string, job string) (atc.Build, error) {
	fake.createJobBuildMutex.Lock()
	ret, specificReturn := fake.createJobBuildReturnsOnCall[len(fake.createJobBuildArgsForCall)]
	fake.createJobBuildArgsForCall = append(fake.createJobBuildArgsForCall, struct {
		pipeline string
		job      string
	}{pipeline, job})
	fake.recordInvocation("CreateJobBuild", []interface{}{pipeline, job})
	fake.createJobBuildMutex.Unlock()
	if fake.CreateJobBuildStub != nil {
		return fake.CreateJobBuildStub(pipeline, job)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.createJobBuildReturns.result1, fake.createJobBuildReturns.result2
}

func (fake *FakePilot) CreateJobBuildCallCount() int {
	fake.createJobBuildMutex.RLock()
	defer fake.createJobBuildMutex.RUnlock()
	return len(fake.createJobBuildArgsForCall)
}

func (fake *FakePilot) CreateJobBuildArgsForCall(i int) (string, string) {
	fake.createJobBuildMutex.RLock()
	defer fake.createJobBuildMutex.RUnlock()
	return fake.createJobBuildArgsForCall[i].pipeline, fake.createJobBuildArgsForCall[i].job
}

func (fake *FakePilot) CreateJobBuildReturns(result1 atc.Build, result2 error) {
	fake.CreateJobBuildStub = nil
	fake.createJobBuildReturns = struct {
		result1 atc.Build
		result2 error
	}{result1, result2}
}

func (fake *FakePilot) CreateJobBuildReturnsOnCall(i int, result1 atc.Build, result2 error) {
	fake.CreateJobBuildStub = nil
	if fake.createJobBuildReturnsOnCall == nil {
		fake.createJobBuildReturnsOnCall = make(map[int]struct {
			result1 atc.Build
			result2 error
		})
	}
	fake.createJobBuildReturnsOnCall[i] = struct {
		result1 atc.Build
		result2 error
	}{result1, result2}
}

func (fake *FakePilot) BuildEvents(job string) (concourse.Events, error) {
	fake.buildEventsMutex.Lock()
	ret, specificReturn := fake.buildEventsReturnsOnCall[len(fake.buildEventsArgsForCall)]
	fake.buildEventsArgsForCall = append(fake.buildEventsArgsForCall, struct {
		job string
	}{job})
	fake.recordInvocation("BuildEvents", []interface{}{job})
	fake.buildEventsMutex.Unlock()
	if fake.BuildEventsStub != nil {
		return fake.BuildEventsStub(job)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.buildEventsReturns.result1, fake.buildEventsReturns.result2
}

func (fake *FakePilot) BuildEventsCallCount() int {
	fake.buildEventsMutex.RLock()
	defer fake.buildEventsMutex.RUnlock()
	return len(fake.buildEventsArgsForCall)
}

func (fake *FakePilot) BuildEventsArgsForCall(i int) string {
	fake.buildEventsMutex.RLock()
	defer fake.buildEventsMutex.RUnlock()
	return fake.buildEventsArgsForCall[i].job
}

func (fake *FakePilot) BuildEventsReturns(result1 concourse.Events, result2 error) {
	fake.BuildEventsStub = nil
	fake.buildEventsReturns = struct {
		result1 concourse.Events
		result2 error
	}{result1, result2}
}

func (fake *FakePilot) BuildEventsReturnsOnCall(i int, result1 concourse.Events, result2 error) {
	fake.BuildEventsStub = nil
	if fake.buildEventsReturnsOnCall == nil {
		fake.buildEventsReturnsOnCall = make(map[int]struct {
			result1 concourse.Events
			result2 error
		})
	}
	fake.buildEventsReturnsOnCall[i] = struct {
		result1 concourse.Events
		result2 error
	}{result1, result2}
}

func (fake *FakePilot) FinishedBuilds(ctx context.Context) <-chan atc.Build {
	fake.finishedBuildsMutex.Lock()
	ret, specificReturn := fake.finishedBuildsReturnsOnCall[len(fake.finishedBuildsArgsForCall)]
	fake.finishedBuildsArgsForCall = append(fake.finishedBuildsArgsForCall, struct {
		ctx context.Context
	}{ctx})
	fake.recordInvocation("FinishedBuilds", []interface{}{ctx})
	fake.finishedBuildsMutex.Unlock()
	if fake.FinishedBuildsStub != nil {
		return fake.FinishedBuildsStub(ctx)
	}
	if specificReturn {
		return ret.result1
	}
	return fake.finishedBuildsReturns.result1
}

func (fake *FakePilot) FinishedBuildsCallCount() int {
	fake.finishedBuildsMutex.RLock()
	defer fake.finishedBuildsMutex.RUnlock()
	return len(fake.finishedBuildsArgsForCall)
}

func (fake *FakePilot) FinishedBuildsArgsForCall(i int) context.Context {
	fake.finishedBuildsMutex.RLock()
	defer fake.finishedBuildsMutex.RUnlock()
	return fake.finishedBuildsArgsForCall[i].ctx
}

func (fake *FakePilot) FinishedBuildsReturns(result1 <-chan atc.Build) {
	fake.FinishedBuildsStub = nil
	fake.finishedBuildsReturns = struct {
		result1 <-chan atc.Build
	}{result1}
}

func (fake *FakePilot) FinishedBuildsReturnsOnCall(i int, result1 <-chan atc.Build) {
	fake.FinishedBuildsStub = nil
	if fake.finishedBuildsReturnsOnCall == nil {
		fake.finishedBuildsReturnsOnCall = make(map[int]struct {
			result1 <-chan atc.Build
		})
	}
	fake.finishedBuildsReturnsOnCall[i] = struct {
		result1 <-chan atc.Build
	}{result1}
}

func (fake *FakePilot) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.uRLMutex.RLock()
	defer fake.uRLMutex.RUnlock()
	fake.pausePipelineMutex.RLock()
	defer fake.pausePipelineMutex.RUnlock()
	fake.unpausePipelineMutex.RLock()
	defer fake.unpausePipelineMutex.RUnlock()
	fake.pauseJobMutex.RLock()
	defer fake.pauseJobMutex.RUnlock()
	fake.unpauseJobMutex.RLock()
	defer fake.unpauseJobMutex.RUnlock()
	fake.createJobBuildMutex.RLock()
	defer fake.createJobBuildMutex.RUnlock()
	fake.buildEventsMutex.RLock()
	defer fake.buildEventsMutex.RUnlock()
	fake.finishedBuildsMutex.RLock()
	defer fake.finishedBuildsMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakePilot) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ flyontime.Pilot = new(FakePilot)
