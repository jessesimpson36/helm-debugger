package branch

import (
	"context"
	"os"

	"github.com/go-delve/delve/service/api"
	"github.com/jessesimpson36/helm-debugger/internal/breakpoints"
	"github.com/jessesimpson36/helm-debugger/internal/dlvcontroller"
	"github.com/jessesimpson36/helm-debugger/internal/frame/delegate"
	"github.com/jessesimpson36/helm-debugger/internal/settings"
)

func Main(cfg *settings.Settings) error {
	ctx := context.Background()
	lines, err := breakpoints.Resolve(cfg.GoRoot)
	if err != nil {
		lines = breakpoints.FallbackLines
	}

	session, err := dlvcontroller.Start(ctx, cfg, os.Stderr)
	if err != nil {
		return err
	}
	defer session.Close()

	frame := breakpoints.GetConditionalFrame(lines)
	frame.WorkingDir = cfg.EffectiveWorkingDir()
	frames := []*delegate.DelegateFrame{frame}
	if err := session.Configure(frames); err != nil {
		return err
	}

	if _, err := session.Client.Restart(false); err != nil {
		return err
	}
	state, err := session.Client.GetState()
	if err != nil {
		return err
	}

	for {
		if state == nil || state.Exited {
			println("Program stopped")
			break
		}

		currentFrame := matchFrame(frames, state)
		if currentFrame == nil {
			state = <-session.Client.Continue()
			continue
		}

		respVars, err := currentFrame.Gather(session.Client)
		if err != nil {
			println("Error gathering variables: " + err.Error())
		} else if result, err := currentFrame.Bind(respVars); err != nil {
			println("Error binding variables: " + err.Error())
		} else if err := result.Display(os.Stdout, false); err != nil {
			println("Error displaying execution unit: " + err.Error())
		}

		state = <-session.Client.Continue()
	}
	return nil
}

func matchFrame(frames []*delegate.DelegateFrame, state *api.DebuggerState) *delegate.DelegateFrame {
	if state == nil || state.CurrentThread == nil || state.CurrentThread.Breakpoint == nil {
		return nil
	}
	name := state.CurrentThread.Breakpoint.Name
	for _, f := range frames {
		for _, bp := range f.Breakpoints {
			if bp.Name == name {
				return f
			}
		}
	}
	return nil
}
