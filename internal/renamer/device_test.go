package renamer

import "testing"

func TestDetectDevice_iPhone(t *testing.T) {
	if got := DetectDevice("IMG_1234.HEIC"); got != "iPhone" {
		t.Errorf("IMG_1234.HEIC = %q, want iPhone", got)
	}
}

func TestDetectDevice_VID(t *testing.T) {
	if got := DetectDevice("VID_20240101_120000.MOV"); got != "iPhone" {
		t.Errorf("VID_... = %q, want iPhone", got)
	}
}

func TestDetectDevice_Pixel(t *testing.T) {
	if got := DetectDevice("PXL_20240315_143022123.jpg"); got != "Pixel" {
		t.Errorf("PXL_... = %q, want Pixel", got)
	}
}

func TestDetectDevice_Screenshot(t *testing.T) {
	if got := DetectDevice("Screenshot 2024-03-15 at 14.30.22.png"); got != "Screenshot" {
		t.Errorf("Screenshot = %q, want Screenshot", got)
	}
}

func TestDetectDevice_Signal(t *testing.T) {
	if got := DetectDevice("signal-2024-03-15-14-30-22.jpg"); got != "Signal" {
		t.Errorf("signal-... = %q, want Signal", got)
	}
}

func TestDetectDevice_WhatsApp(t *testing.T) {
	if got := DetectDevice("IMG-20240315-WA0001.jpg"); got != "WhatsApp" {
		t.Errorf("IMG-...-WA... = %q, want WhatsApp", got)
	}
}

func TestDetectDevice_Camera(t *testing.T) {
	if got := DetectDevice("DSC_0456.JPG"); got != "Camera" {
		t.Errorf("DSC_... = %q, want Camera", got)
	}
}

func TestDetectDevice_Motion(t *testing.T) {
	if got := DetectDevice("MVIMG_20240315_143022.jpg"); got != "Motion" {
		t.Errorf("MVIMG_... = %q, want Motion", got)
	}
}

func TestDetectDevice_Unknown(t *testing.T) {
	if got := DetectDevice("random_photo.jpg"); got != "Unknown" {
		t.Errorf("random_photo = %q, want Unknown", got)
	}
}

func TestDetectDevice_CaseInsensitive(t *testing.T) {
	cases := map[string]string{
		"img_1234.jpg":       "iPhone",
		"SCREENSHOT_1.png":   "Screenshot",
		"pxl_123.jpg":        "Pixel",
		"dsc_001.jpg":        "Camera",
		"SIGNAL-2024-01.jpg": "Signal",
	}
	for name, want := range cases {
		if got := DetectDevice(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestDetectDevice_Unicode(t *testing.T) {
	if got := DetectDevice("фото_с_отпуска.jpg"); got != "Unknown" {
		t.Errorf("unicode = %q, want Unknown", got)
	}
}
