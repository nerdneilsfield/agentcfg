package all

import (
	"strings"
	"testing"

	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
	"agentcfg/internal/target"
)

func authTypeConfig(authType ir.AuthType) ir.Config {
	return ir.Config{Providers: []ir.Provider{{
		ID:       "test",
		Protocol: ir.ProtocolOpenAICompletions,
		BaseURL:  "https://example.com/v1",
		APIKey:   ir.HeaderValue{FromEnv: "AGENTCFG_TEST_KEY"},
		AuthType: authType,
		Models:   []ir.Model{{ID: "model"}},
	}}}
}

func mappedAuthTypes(emitter target.Target) map[ir.AuthType]bool {
	mapped := map[ir.AuthType]bool{ir.AuthTypeOfficial: true}
	if m, ok := emitter.(target.AuthTypeMapper); ok {
		for _, authType := range m.MappedAuthTypes() {
			mapped[authType] = true
		}
	}
	return mapped
}

// Every target either maps auth_type or says it does not. Silently ignoring it
// would let the emitted config authenticate differently than the IR asked for.
func TestAuthTypeIsNeverSilentlyIgnored(t *testing.T) {
	for _, id := range target.IDs() {
		t.Run(id, func(t *testing.T) {
			emitter, _ := target.Lookup(id)
			for _, authType := range []ir.AuthType{ir.AuthTypeBearer, ir.AuthTypeNone} {
				diags := target.AuthTypeDiagnostics(emitter, authTypeConfig(authType))
				if mappedAuthTypes(emitter)[authType] {
					if diag.HasErrors(diags) {
						t.Fatalf("%s maps auth_type %s but reported: %v", id, authType, diags)
					}
					continue
				}
				if !diag.HasErrors(diags) {
					t.Fatalf("%s silently accepted auth_type: %s", id, authType)
				}
				if !strings.Contains(diags[0].Message, "does not implement auth_type") {
					t.Fatalf("%s unexpected diagnostic: %v", id, diags)
				}
			}
		})
	}
}

// The default must stay a no-op so existing configurations are unaffected.
func TestOfficialAuthTypeIsNoOp(t *testing.T) {
	for _, id := range target.IDs() {
		t.Run(id, func(t *testing.T) {
			emitter, _ := target.Lookup(id)
			for _, authType := range []ir.AuthType{"", ir.AuthTypeOfficial} {
				diags := target.AuthTypeDiagnostics(emitter, authTypeConfig(authType))
				if diag.HasErrors(diags) {
					t.Fatalf("%s rejected auth_type %q: %v", id, authType, diags)
				}
			}
		})
	}
}
