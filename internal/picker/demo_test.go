package picker

import (
	"os"
	"testing"
)

func TestPickerDemo(t *testing.T) {
	if os.Getenv("SSM_PICKER_DEMO") == "" {
		t.Skip("set SSM_PICKER_DEMO=1 to open the picker interactively")
	}

	items := append(sampleTargets(),
		mkEC2("ldap-01", "i-0e5f60718293a4b5c", "Online"),
		mkEC2("app-02", "i-0c3d4e5f60718293a", "Online"),
		mkEC2("micro-01", "i-0d4e5f60718293a4b", "Online"),
		mkEC2("mail-01", "i-0f60718293a4b5c6d", "ConnectionLost"),
		mkECS("prod", "worker", "app", "9f8e7d6c", "", "RUNNING", false, ""),
		mkECS("prod", "batch", "app", "5c4b3a29", "r9", "PENDING", true, "PENDING"),
	)

	got, err := Pick(items, Options{
		Noun:    "target",
		Region:  "eu-west-1",
		Profile: "prod",
		NoFzf:   os.Getenv("SSM_PICKER_NOFZF") != "",
	})
	if err != nil {
		t.Fatalf("Pick: %v", err)
	}
	t.Logf("selected %s -> SSM target %s", got.Label(), got.SSMTarget())
}
