package scan

import "testing"

// A helper in the app-hosted bundle is test code though its name does not say so —
// mindlensTests/KeychainProbe.swift was read as production until this case existed.
func TestIsTestCoversEveryTestLocation(t *testing.T) {
	for path, want := range map[string]bool{
		"Packages/MindlensKit/Tests/CoreTests/AppErrorTests.swift":    true,
		"Packages/MindlensKit/Sources/TestSupport/Fakes.swift":        true,
		"mindlensTests/KeychainProbe.swift":                           true,
		"mindlensUITests/LaunchSmokeTests.swift":                      true,
		"Packages/MindlensKit/Sources/Persistence/KeychainItem.swift": false,
		"mindlens/RootView.swift":                                     false,
	} {
		if got := (ChangedFile{Path: path}).IsTest(); got != want {
			t.Errorf("IsTest(%s) = %v, want %v", path, got, want)
		}
	}
}
