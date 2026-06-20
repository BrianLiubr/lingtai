package migrate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// writeAgentInitPortal writes an agent init.json under <lingtaiDir>/<name>/init.json.
// Named with a suffix to avoid collision with any future TUI-ported helper.
func writeAgentInitPortal(t *testing.T, lingtaiDir, name, body string) string {
	t.Helper()
	dir := filepath.Join(lingtaiDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	p := filepath.Join(dir, "init.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func readAgentSkillsPortal(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("patched init.json is not valid json: %v\n%s", err, data)
	}
	manifest, ok := doc["manifest"].(map[string]interface{})
	if !ok {
		t.Fatalf("manifest missing or not a map: %s", data)
	}
	caps, ok := manifest["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatalf("capabilities missing or not a map: %s", data)
	}
	skills, ok := caps["skills"].(map[string]interface{})
	if !ok {
		t.Fatalf("skills missing or not a map: %s", data)
	}
	return skills
}

func readManifestPortal(t *testing.T, initPath string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(initPath)
	if err != nil {
		t.Fatalf("read %s: %v", initPath, err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal %s: %v\n%s", initPath, err, data)
	}
	manifest, ok := m["manifest"].(map[string]interface{})
	if !ok {
		t.Fatalf("manifest missing in %s", initPath)
	}
	return manifest
}

func toStringSlicePortal(t *testing.T, raw interface{}) []string {
	t.Helper()
	items, ok := raw.([]interface{})
	if !ok {
		t.Fatalf("allowed is %T, want []interface{}", raw)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			t.Fatalf("allowed entry is %T, want string", item)
		}
		out = append(out, s)
	}
	return out
}

func TestPortalM038_PatchesMultipleAgents(t *testing.T) {
	lingtaiDir := t.TempDir()
	orch := writeAgentInitPortal(t, lingtaiDir, "orch",
		`{"manifest":{"capabilities":{"skills":{"library_limit":42},"web_search":{"provider":"zhipu"}}}}`)
	scout := writeAgentInitPortal(t, lingtaiDir, "scout",
		`{"manifest":{"capabilities":{"bash":{"yolo":true}}}}`)

	if err := migrateAgentInitSkillsPaths(lingtaiDir); err != nil {
		t.Fatal(err)
	}

	orchSkills := readAgentSkillsPortal(t, orch)
	if !reflect.DeepEqual(orchSkills["paths"], defaultPresetSkillsPaths) {
		t.Fatalf("orch skills.paths = %#v, want %#v", orchSkills["paths"], defaultPresetSkillsPaths)
	}
	if got := orchSkills["library_limit"]; got != float64(42) {
		t.Fatalf("orch existing skills config overwritten: %#v", orchSkills)
	}

	scoutSkills := readAgentSkillsPortal(t, scout)
	if !reflect.DeepEqual(scoutSkills["paths"], defaultPresetSkillsPaths) {
		t.Fatalf("scout skills.paths = %#v, want %#v", scoutSkills["paths"], defaultPresetSkillsPaths)
	}

	data, _ := os.ReadFile(orch)
	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	caps := doc["manifest"].(map[string]interface{})["capabilities"].(map[string]interface{})
	ws, ok := caps["web_search"].(map[string]interface{})
	if !ok || ws["provider"] != "zhipu" {
		t.Fatalf("web_search entry damaged: %#v", caps["web_search"])
	}
}

func TestPortalM038_CreatesMissingCapabilitiesMap(t *testing.T) {
	lingtaiDir := t.TempDir()
	path := writeAgentInitPortal(t, lingtaiDir, "orch", `{"manifest":{"llm":{"provider":"custom"}}}`)

	if err := migrateAgentInitSkillsPaths(lingtaiDir); err != nil {
		t.Fatal(err)
	}

	skills := readAgentSkillsPortal(t, path)
	if !reflect.DeepEqual(skills["paths"], defaultPresetSkillsPaths) {
		t.Fatalf("skills.paths = %#v, want %#v", skills["paths"], defaultPresetSkillsPaths)
	}
}

func TestPortalM038_PreservesCustomPaths(t *testing.T) {
	lingtaiDir := t.TempDir()
	content := `{"manifest":{"capabilities":{"skills":{"paths":["./custom-skills"]}}}}`
	path := writeAgentInitPortal(t, lingtaiDir, "orch", content)

	if err := migrateAgentInitSkillsPaths(lingtaiDir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Fatalf("init.json with custom paths was rewritten: %s", data)
	}
}

func TestPortalM038_SkipsMalformedAndNonMapCases(t *testing.T) {
	lingtaiDir := t.TempDir()
	malformed := writeAgentInitPortal(t, lingtaiDir, "broken", `{"manifest":`)
	nonMapCaps := writeAgentInitPortal(t, lingtaiDir, "weird-caps",
		`{"manifest":{"capabilities":["skills"]}}`)
	good := writeAgentInitPortal(t, lingtaiDir, "zz-good", `{"manifest":{"capabilities":{}}}`)

	if err := migrateAgentInitSkillsPaths(lingtaiDir); err != nil {
		t.Fatal(err)
	}

	for name, path := range map[string]string{
		"malformed": malformed, "non-map capabilities": nonMapCaps,
	} {
		before := map[string]string{
			"malformed":            `{"manifest":`,
			"non-map capabilities": `{"manifest":{"capabilities":["skills"]}}`,
		}[name]
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != before {
			t.Fatalf("%s init.json should be unchanged, got: %s", name, data)
		}
	}

	skills := readAgentSkillsPortal(t, good)
	if !reflect.DeepEqual(skills["paths"], defaultPresetSkillsPaths) {
		t.Fatalf("valid sibling not patched: %#v", skills["paths"])
	}
}
