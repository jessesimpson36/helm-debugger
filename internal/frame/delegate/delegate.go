package delegate

import (
	"strings"

	"github.com/go-delve/delve/service/rpc2"
	"github.com/jessesimpson36/helm-debugger/internal/frame"
	"github.com/jessesimpson36/helm-debugger/internal/frame/renderedmanifest"
	"github.com/jessesimpson36/helm-debugger/internal/frame/templateframe"
)

func GetFrameType(undeterminedFrame *frame.Frame) string {
	for _, breakpoint := range undeterminedFrame.Breakpoints {
		if strings.HasPrefix(breakpoint.Name, "conditional") {
			return "conditional"
		} else if strings.HasPrefix(breakpoint.Name, "line") {
			return "line"
		} else if strings.HasPrefix(breakpoint.Name, "rendered") {
			return "rendered"
		}
	}
	return ""
}

type DelegateFrame frame.Frame

func (d *DelegateFrame) Gather(client *rpc2.RPCClient) (map[string]string, error) {
	dFrame := &frame.Frame{
		Breakpoints: d.Breakpoints,
		ReqVars:     d.ReqVars,
		Mapper:      d.Mapper,
	}
	frameType := GetFrameType(dFrame)
	switch frameType {
	case "conditional", "line":
		tFrame := templateframe.TemplateFrame{
			Breakpoints: d.Breakpoints,
			ReqVars:     d.ReqVars,
			Mapper:      d.Mapper,
		}
		return tFrame.Gather(client)
	case "rendered":
		rmFrame := renderedmanifest.RenderedManifestFrame{
			Breakpoints: d.Breakpoints,
			ReqVars:     d.ReqVars,
			Mapper:      d.Mapper,
		}
		return rmFrame.Gather(client)
	default:
		panic("invalid frame type")
	}
}

func (d *DelegateFrame) Bind(respVars map[string]string) (*frame.BindResult, error) {
	dFrame := &frame.Frame{
		Breakpoints: d.Breakpoints,
		ReqVars:     d.ReqVars,
		Mapper:      d.Mapper,
	}
	frameType := GetFrameType(dFrame)
	switch frameType {
	case "conditional", "line":
		tFrame := templateframe.TemplateFrame{
			Breakpoints: d.Breakpoints,
			ReqVars:     d.ReqVars,
			Mapper:      d.Mapper,
		}
		return tFrame.Bind(respVars)
	case "rendered":
		rmFrame := renderedmanifest.RenderedManifestFrame{
			Breakpoints: d.Breakpoints,
			ReqVars:     d.ReqVars,
			Mapper:      d.Mapper,
		}
		return rmFrame.Bind(respVars)
	default:
		panic("invalid frame type")
	}
}
