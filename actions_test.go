package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestPlayerLaunchURL(t *testing.T) {
	for _, tc := range []struct{ name, url, channel, want string }{
		{"empty website channel", "roblox-player:1+gameinfo:ticket+channel:+other:keep", "zterrainearlyaccess", "roblox-player:1+gameinfo:ticket+channel:zterrainearlyaccess+other:keep"},
		{"missing channel", "roblox-player:1+gameinfo:ticket", "zterrainearlyaccess", "roblox-player:1+gameinfo:ticket+channel:zterrainearlyaccess"},
		{"public channel", "roblox-player:1+channel:production", "zterrainearlyaccess", "roblox-player:1+channel:zterrainearlyaccess"},
		{"live install", "roblox-player:1+channel:", "", "roblox-player:1+channel:"},
		{"deep link", "roblox://placeId=123", "zterrainearlyaccess", "roblox://placeId=123"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := playerLaunchURL(tc.url, tc.channel); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDetectPlayerUpdaterReplacement(t *testing.T) {
	home := t.TempDir()
	info := filepath.Join(playerApp(home), "Contents", "Info.plist")
	receipt := filepath.Join(home, ".local/share/evergreen/bundle-checksum")
	for _, dir := range []string{filepath.Dir(info), filepath.Dir(receipt)} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	beta := []byte("beta Info.plist")
	if err := os.WriteFile(info, beta, 0644); err != nil {
		t.Fatal(err)
	}
	if installedBuildIntact(home) {
		t.Fatal("legacy installation needs verification")
	}
	if err := os.WriteFile(receipt, []byte(fmt.Sprintf("%x", sha256.Sum256(beta))), 0644); err != nil {
		t.Fatal(err)
	}
	if !installedBuildIntact(home) {
		t.Fatal("unchanged beta was rejected")
	}
	if err := os.WriteFile(info, []byte("production Info.plist"), 0644); err != nil {
		t.Fatal(err)
	}
	if installedBuildIntact(home) {
		t.Fatal("updater replacement was missed")
	}
}
