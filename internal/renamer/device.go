package renamer

import "strings"

// DetectDevice определяет устройство/источник по имени файла на основе эвристик.
func DetectDevice(name string) string {
	base := strings.ToLower(name)
	switch {
	case strings.HasPrefix(base, "img_"):
		return "iPhone"
	case strings.HasPrefix(base, "vid_"):
		return "iPhone"
	case strings.HasPrefix(base, "pxl_"):
		return "Pixel"
	case strings.HasPrefix(base, "dsc_"):
		return "Camera"
	case strings.HasPrefix(base, "screenshot"):
		return "Screenshot"
	case strings.HasPrefix(base, "signal-"):
		return "Signal"
	case strings.Contains(base, "-wa"):
		return "WhatsApp"
	case strings.HasPrefix(base, "mvimg_"):
		return "Motion"
	default:
		return "Unknown"
	}
}
