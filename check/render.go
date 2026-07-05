package check

import (
	"fmt"
	"strings"

	"github.com/beihehele/subs-check/config"
	proxyutils "github.com/beihehele/subs-check/proxy"
)

// RenderName 根据 Result 的结构化字段构造展示名。
//
// 这是整个项目唯一的"节点名生成"出口,纯函数:
//   - 无 I/O,无 goroutine
//   - 不读写 proxy map 的 name 字段,不修改 Result
//   - 仅依赖传入的 Result 和 config.GlobalConfig
//
// includeSpeed 为 true 时追加速度标签,只在最终输出 all.yaml 时用。
// filter 阶段应该传 false,因为此时尚未测速。
// rename-node 开启且 seq<=0 时不带 _N 后缀,供 filter 预览;最终序号由 AssignDisplayNames 分配。
func RenderName(r Result, includeSpeed bool) string {
	return renderName(r, includeSpeed, 0)
}

// AssignDisplayNames writes final display names into results.
// When rename-node is enabled, _N suffixes are assigned once in region-sorted output order.
func AssignDisplayNames(results []Result) {
	if len(results) == 0 {
		return
	}

	regionSeq := make(map[string]int)
	for _, idx := range OutputOrderIndices(results) {
		r := results[idx]
		if r.Proxy == nil {
			continue
		}

		var name string
		if config.GlobalConfig.RenameNode {
			key := proxyutils.RenameSeqKey(r.Country)
			regionSeq[key]++
			name = renderName(r, true, regionSeq[key])
		} else {
			name = renderName(r, true, 0)
		}
		results[idx].Proxy["name"] = name
	}
}

func renderName(r Result, includeSpeed bool, seq int) string {
	// RenameNode 是"强覆盖合约":只要开了就用 FormatRename 覆盖原名,
	// Country 为空时走 ❓Other 兜底。
	// 这样能确保上游订阅里已有的 |speed|media 尾缀不会透传进来再被叠加,
	// 否则在 IP 查询失败(免费节点常见)的节点上会出现重复标签。
	var base string
	if config.GlobalConfig.RenameNode {
		base = config.GlobalConfig.NodePrefix + proxyutils.FormatRename(r.Country, seq)
	} else if r.Proxy != nil {
		if n, ok := r.Proxy["name"].(string); ok {
			base = strings.TrimSpace(n)
		}
	}

	var tags []string
	if includeSpeed && config.GlobalConfig.SpeedTestUrl != "" && r.Speed > 0 {
		tags = append(tags, formatSpeedTag(r.Speed))
	}

	for _, plat := range config.GlobalConfig.Platforms {
		if tag := mediaTagFor(plat, &r); tag != "" {
			tags = append(tags, tag)
		}
	}

	if r.Proxy != nil {
		if t, ok := r.Proxy["sub_tag"].(string); ok && t != "" {
			tags = append(tags, t)
		}
	}

	if len(tags) == 0 {
		return base
	}
	return base + "|" + strings.Join(tags, "|")
}

// mediaTagFor 返回单个平台的展示标签,未命中返回空字符串。
// 新增平台时只需在这里加一个 case 和对应的 Result 字段。
func mediaTagFor(plat string, r *Result) string {
	switch plat {
	case "openai":
		if r.Openai != nil {
			if r.Openai.Full {
				if r.Openai.Region != "" {
					return fmt.Sprintf("GPT⁺-%s", r.Openai.Region)
				}
				return "GPT⁺"
			}
			if r.Openai.Web {
				if r.Openai.Region != "" {
					return fmt.Sprintf("GPT-%s", r.Openai.Region)
				}
				return "GPT"
			}
		}
	case "netflix":
		if r.Netflix != nil {
			if r.Netflix.Full {
				if r.Netflix.Region != "" {
					return fmt.Sprintf("NF-%s", r.Netflix.Region)
				}
				return "NF"
			}
			if r.Netflix.OriginalsOnly {
				return "NF"
			}
		}
	case "disney":
		if r.Disney {
			return "D+"
		}
	case "gemini":
		if r.Gemini != "" {
			return fmt.Sprintf("GM-%s", r.Gemini)
		}
	case "claude":
		if r.Claude != "" {
			return fmt.Sprintf("CL-%s", r.Claude)
		}
	case "spotify":
		if r.Spotify != "" {
			return fmt.Sprintf("SP-%s", r.Spotify)
		}
	case "iprisk":
		if r.IPRisk != "" {
			return r.IPRisk
		}
	case "youtube":
		if r.Youtube != "" {
			return fmt.Sprintf("YT-%s", r.Youtube)
		}
	case "tiktok":
		if r.TikTok != "" {
			return fmt.Sprintf("TK-%s", r.TikTok)
		}
	}
	return ""
}

// formatSpeedTag 把测速结果(KB/s)格式化为展示字符串。
//
//	<1024 → "NKB/s"
//	>=1024 → "X.XMB/s"
func formatSpeedTag(speed int) string {
	if speed < 1024 {
		return fmt.Sprintf("%dKB/s", speed)
	}
	return fmt.Sprintf("%.1fMB/s", float64(speed)/1024)
}
