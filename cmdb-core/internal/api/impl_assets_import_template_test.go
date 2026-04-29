package api

import (
	"encoding/csv"
	"strings"
	"testing"
)

// importTemplateForLang round-trip check: every Chinese header used here
// must be one that the ingestion-engine FIELD_ALIASES map recognises,
// otherwise re-uploading a downloaded template would fail to import.
func TestImportTemplateForLang_LanguageMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		lang             string
		wantSuffix       string
		wantHeaderTokens []string // strings that must appear in the CSV header line
		wantExampleHas   string   // a substring expected somewhere in the example row
	}{
		{
			lang:             "en",
			wantSuffix:       "",
			wantHeaderTokens: []string{"asset_tag", "name", "type", "ip_address", "warranty_end"},
			wantExampleHas:   "Production Server 01",
		},
		{
			lang:             "zh-TW",
			wantSuffix:       "-zh-TW",
			wantHeaderTokens: []string{"資產編號", "名稱", "類型", "序列號", "保固到期"},
			wantExampleHas:   "生產伺服器",
		},
		{
			lang:             "zh-CN",
			wantSuffix:       "-zh-CN",
			wantHeaderTokens: []string{"资产编号", "名称", "类型", "序列号", "质保到期"},
			wantExampleHas:   "生产服务器",
		},
		{
			// Unknown lang must fall back to en (current behaviour) so the
			// caller never gets a 4xx for a typo.
			lang:             "klingon",
			wantSuffix:       "",
			wantHeaderTokens: []string{"asset_tag", "name", "type"},
			wantExampleHas:   "Production Server 01",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.lang, func(t *testing.T) {
			t.Parallel()
			header, example, suffix := importTemplateForLang(tc.lang)
			if suffix != tc.wantSuffix {
				t.Errorf("suffix = %q, want %q", suffix, tc.wantSuffix)
			}
			for _, tok := range tc.wantHeaderTokens {
				if !strings.Contains(header, tok) {
					t.Errorf("header missing token %q\nheader: %s", tok, header)
				}
			}
			if !strings.Contains(example, tc.wantExampleHas) {
				t.Errorf("example missing %q\nexample: %s", tc.wantExampleHas, example)
			}
			// CSV sanity: header and example must have the same column count.
			// Parse via encoding/csv so quoted commas (e.g. tags="prod,crit")
			// don't inflate the count.
			parse := func(line string) ([]string, error) {
				return csv.NewReader(strings.NewReader(line)).Read()
			}
			hcols, err := parse(strings.TrimRight(header, "\n"))
			if err != nil {
				t.Fatalf("parse header: %v", err)
			}
			ecols, err := parse(strings.TrimRight(example, "\n"))
			if err != nil {
				t.Fatalf("parse example: %v", err)
			}
			if len(hcols) != len(ecols) {
				t.Errorf("column count mismatch: header=%d example=%d", len(hcols), len(ecols))
			}
			if len(hcols) != 26 {
				t.Errorf("expected 26 columns, got %d", len(hcols))
			}
		})
	}
}
