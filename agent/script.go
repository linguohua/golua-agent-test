package agent

import (
	log "github.com/sirupsen/logrus"
	lua "github.com/yuin/gopher-lua"
)

type ScriptEvent interface {
	evtType() string
}

type Script struct {
	fileMD5 string

	eventsChan chan ScriptEvent

	state *lua.LState

	modTable *lua.LTable

	timerModule *TimerModule
}

func (s *Script) events() <-chan ScriptEvent {
	return s.eventsChan
}

func (s *Script) pushEvt(evt ScriptEvent) {
	s.eventsChan <- evt
}

func (s *Script) handleEvent(evt ScriptEvent) {
	switch evt.evtType() {
	case "timer":
		e := evt.(*TimerEvent)
		if e != nil && s.timerModule.hasTimer(e.tag) {
			s.callModFunction1(e.callback, lua.LString(e.tag))
		}
	}
}

func newScript(scriptFileMD5 string, fileContent []byte) *Script {
	s := &Script{
		fileMD5:    scriptFileMD5,
		eventsChan: make(chan ScriptEvent, 64),
	}

	s.state = lua.NewState()

	if len(fileContent) > 0 {
		s.load(fileContent)
	}

	return s
}

func (s *Script) start() {
	ls := s.state
	s.timerModule = newTimerModule(s)
	ls.PreloadModule("timer", s.timerModule.loader)

	if s.modTable != nil {
		// exec 'start' funciton in lua mod
		s.callModFunction0("start")
	}
}

func (s *Script) hasLuaFunction(funcName string) bool {
	if s.modTable != nil {
		fn := s.state.GetField(s.modTable, funcName)
		return fn != nil
	}

	return false
}

func (s *Script) callModFunction0(funcName string) {
	ls := s.state
	fn := ls.GetField(s.modTable, funcName)
	if fn != nil {
		ls.Push(fn)
		err := ls.PCall(0, lua.MultRet, nil)
		if err != nil {
			log.Errorf("callModFunction0 %s failed:%v", funcName, err)
		}
	}
}

func (s *Script) callModFunction1(funcName string, param0 lua.LValue) {
	ls := s.state
	fn := ls.GetField(s.modTable, funcName)
	if fn != nil {
		ls.Push(fn)
		ls.Push(param0)
		err := ls.PCall(1, lua.MultRet, nil)
		if err != nil {
			log.Errorf("callModFunction1 %s failed:%v", funcName, err)
		}
	}
}

func (s *Script) stop() {
	ls := s.state
	if s.modTable != nil {
		// exec 'stop' funciton in lua mod
		s.callModFunction0("stop")
	}

	ls.Close()
	s.state = nil
	s.modTable = nil
	s.timerModule.clear()
	s.timerModule = nil
}

func (s *Script) load(fileContent []byte) {
	ls := s.state
	fn, err := ls.LoadString(string(fileContent))
	if err != nil {
		log.Errorf("lstate load string failed:%v", err)
		return
	}

	ls.Push(fn)
	err = ls.PCall(0, lua.MultRet, nil)
	if err != nil {
		log.Errorf("lstate load string failed:%v", err)
		return
	}

	s.modTable = ls.ToTable(-1)
}
