package docker

import "sync"

type InstallState struct {
	Running bool     `json:"running"`
	Done    bool     `json:"done"`
	Error   string   `json:"error,omitempty"`
	Logs    []string `json:"logs"`
}

var (
	state InstallState
	mutex sync.Mutex
)

func GetState() InstallState {
	mutex.Lock()
	defer mutex.Unlock()
	return state
}

func setRunning() {
	mutex.Lock()
	state = InstallState{
		Running: true,
		Done:    false,
		Logs:    []string{},
	}
	mutex.Unlock()
}

func appendLog(line string) {
	mutex.Lock()
	state.Logs = append(state.Logs, line)
	mutex.Unlock()
}

func setDone(err error) {
	mutex.Lock()
	state.Running = false
	state.Done = true
	if err != nil {
		state.Error = err.Error()
	}
	mutex.Unlock()
}
