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
func RenderName(r Result, includeSpeed bool) string {
	return RenderNameParts(r, includeSpeed).String()
}

// NameParts is the structured form of a display name; String() joins it as
// "base|speed|media...|sub_tag". Rendering is pure; final sequence numbers are
// assigned only by AssignDisplayNames in output order.
type NameParts struct {
	Stable   bool // omit changing tags from the subscription identity
	Base     string
	SpeedTag string     // set when includeSpeed and the node has a speed
	Media    []MediaTag // config.Platforms order; misses kept with an empty Tag
	SubTag   string
}

// MediaTag is one platform result; Status distinguishes failure from uncertainty.
type MediaTag struct {
	Platform string `json:"platform"`
	Tag      string `json:"tag"`
	Status   string `json:"status,omitempty"`
}

func (p NameParts) String() string {
	if p.Stable {
		return p.Base
	}
	return p.TaggedString()
}

func (p NameParts) TaggedString() string {
	var tags []string
	if p.SpeedTag != "" {
		tags = append(tags, p.SpeedTag)
	}
	for _, m := range p.Media {
		switch m.Platform {
		case "disney", "gemini", "claude", "spotify":
			continue
		}
		if m.Tag != "" {
			tags = append(tags, m.Tag)
		}
	}
	if p.SubTag != "" {
		tags = append(tags, p.SubTag)
	}
	if len(tags) == 0 {
		return p.Base
	}
	return p.Base + "|" + strings.Join(tags, "|")
}

// RenderNameParts is the structured form of RenderName.
func RenderNameParts(r Result, includeSpeed bool) NameParts {
	return renderNameParts(r, includeSpeed, 0)
}

func renderNameParts(r Result, includeSpeed bool, seq int) NameParts {
	var p NameParts
	ResolveRegion(&r)

	// 1. base 名字
	// RenameNode 是"强覆盖合约":只要开了就用 FormatRename 覆盖原名,
	// 出口地区未知时尝试原名主体推断，再走 ❓Other 兜底；预览时不分配序号。
	// 这样能确保上游订阅里已有的 |speed|media 尾缀不会透传进来再被叠加,
	// 否则在 IP 查询失败(免费节点常见)的节点上会出现重复标签。
	if config.GlobalConfig.RenameNode {
		p.Base = config.GlobalConfig.NodePrefix + proxyutils.FormatRename(r.Region, seq)
		if config.GlobalConfig.NameMode == "stable" {
			p.Stable = true
			// A fixed SC prefix avoids name churn when exit geolocation changes.
			id := proxyutils.NodeID(r.Proxy)
			if len(id) > 16 {
				id = id[:16]
			}
			p.Base = config.GlobalConfig.NodePrefix + "SC-" + id
		}
	} else if r.Proxy != nil {
		if n, ok := r.Proxy["name"].(string); ok {
			p.Base = strings.TrimSpace(n)
		}
	}

	// 2. 速度标签(仅 includeSpeed 且有速度时追加,放在媒体标签之前以保持与旧版相同的展示顺序)
	if includeSpeed && config.GlobalConfig.SpeedTestUrl != "" && r.Speed > 0 {
		p.SpeedTag = formatSpeedTag(r.Speed)
	}

	// 3. 按 config.Platforms 顺序收集媒体标签
	for _, plat := range config.GlobalConfig.Platforms {
		tag, status := mediaTagFor(plat, &r), r.MediaStates[plat]
		if status == "" {
			status = "unknown"
			if tag != "" {
				status = "passed"
			}
		}
		p.Media = append(p.Media, MediaTag{Platform: plat, Tag: tag, Status: status})
	}

	// 4. sub_tag 追加到最后
	if r.Proxy != nil {
		if t, ok := r.Proxy["sub_tag"].(string); ok && t != "" {
			p.SubTag = t
		}
	}

	return p
}

// AssignDisplayNames assigns final names once in region-sorted output order.
// Returned parts correspond to the original result indices, so snapshots and
// subscriptions share exactly the same names without rendering mutated names.
func AssignDisplayNames(results []Result) []NameParts {
	for i := range results {
		ResolveRegion(&results[i])
	}
	parts := make([]NameParts, len(results))
	ids := make(map[string]string)
	collisions := make(map[string]bool)
	for _, r := range results {
		id := proxyutils.NodeID(r.Proxy)
		if len(id) < 16 {
			continue
		}
		short := id[:16]
		if prev, ok := ids[short]; ok && prev != id {
			collisions[short] = true
		}
		ids[short] = id
	}
	regionSeq := make(map[string]int)
	for _, idx := range OutputOrderIndices(results) {
		r := results[idx]
		if r.Proxy == nil {
			continue
		}
		seq := 0
		if config.GlobalConfig.RenameNode {
			key := proxyutils.RenameSeqKey(r.Region)
			regionSeq[key]++
			seq = regionSeq[key]
		}
		parts[idx] = renderNameParts(r, true, seq)
		if parts[idx].Stable {
			id := proxyutils.NodeID(r.Proxy)
			if len(id) >= 16 && collisions[id[:16]] {
				parts[idx].Base = config.GlobalConfig.NodePrefix + "SC-" + id
			}
		}
		results[idx].Proxy["name"] = parts[idx].String()
	}
	return parts
}

// mediaTagFor 返回单个平台的展示标签,未命中返回空字符串。
// 新增平台时只需在这里加一个 case 和对应的 Result 字段。
func mediaTagFor(plat string, r *Result) string {
	switch plat {
	case "telegram":
		if r.Telegram != nil && r.Telegram.Reachable {
			return "TG"
		}
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
		if r.Disney != nil && r.Disney.Unlocked {
			if r.Disney.Region != "" {
				return fmt.Sprintf("D+-%s", r.Disney.Region)
			}
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
