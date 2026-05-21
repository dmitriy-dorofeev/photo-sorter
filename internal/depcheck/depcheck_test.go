package depcheck

import (
	"strings"
	"testing"
)

func TestCheckAll(t *testing.T) {
	results := CheckAll()
	if len(results) == 0 {
		t.Fatal("CheckAll вернул пустой результат")
	}
	for _, r := range results {
		if r.Name == "" {
			t.Error("пустое имя зависимости")
		}
	}
}

func TestResultsFilterMissing(t *testing.T) {
	rr := Results{
		{Name: "a", Status: StatusOK},
		{Name: "b", Status: StatusMissing, Optional: true},
		{Name: "c", Status: StatusMissing, Optional: false},
	}
	missing := rr.FilterMissing()
	if len(missing) != 2 {
		t.Fatalf("ожидалось 2 missing, получено %d", len(missing))
	}
	if missing[0].Name != "b" || missing[1].Name != "c" {
		t.Error("неверный список missing")
	}
}

func TestResultsHasMissingRequired(t *testing.T) {
	if (Results{{Status: StatusMissing, Optional: true}}).HasMissingRequired() {
		t.Error("optional missing не должен считаться required")
	}
	if !(Results{{Status: StatusMissing, Optional: false}}).HasMissingRequired() {
		t.Error("required missing должен быть найден")
	}
}

func TestRenderText(t *testing.T) {
	rr := Results{
		{Name: "exiftool", Status: StatusOK, Details: "/usr/bin/exiftool"},
		{Name: "ONNX Runtime", Status: StatusMissing, Optional: true, Details: "not found"},
	}
	txt := rr.RenderText()
	if !strings.Contains(txt, "exiftool") {
		t.Error("RenderText не содержит exiftool")
	}
	if !strings.Contains(txt, "ONNX Runtime") {
		t.Error("RenderText не содержит ONNX Runtime")
	}
	if !strings.Contains(txt, "✅") {
		t.Error("RenderText не содержит иконку OK")
	}
	if !strings.Contains(txt, "ОТСУТСТВУЕТ") {
		t.Error("RenderText не содержит статус ОТСУТСТВУЕТ")
	}
}
