package dev_caches

import (
	"os"
	"path/filepath"
)

// Cache represents a regenerable developer cache directory.
type Cache struct {
	Name      string
	Path      string
	SizeBytes int64
}

// Scan finds common dev caches that are safe to clear.
func Scan(home string) []Cache {
	candidates := []struct {
		name string
		path string
	}{
		{"npm cache", filepath.Join(home, ".npm")},
		{"pnpm store", filepath.Join(home, "Library", "pnpm", "store")},
		{"pip cache", filepath.Join(home, "Library", "Caches", "pip")},
		{"pip3 cache", filepath.Join(home, ".cache", "pip")},
		{"Homebrew downloads", "/usr/local/Homebrew/Library/Homebrew/vendor"},
		{"Homebrew cache", filepath.Join(home, "Library", "Caches", "Homebrew")},
		{"cargo registry", filepath.Join(home, ".cargo", "registry")},
		{"Go build cache", filepath.Join(home, "Library", "Caches", "go-build")},
		{"Go build cache (XDG)", filepath.Join(home, ".cache", "go-build")},
		{"Gradle cache", filepath.Join(home, ".gradle", "caches")},
		{"Maven local repo", filepath.Join(home, ".m2", "repository")},
		{"CocoaPods cache", filepath.Join(home, "Library", "Caches", "CocoaPods")},
		{"Xcode DerivedData", filepath.Join(home, "Library", "Developer", "Xcode", "DerivedData")},
		{"iOS DeviceSupport", filepath.Join(home, "Library", "Developer", "Xcode", "iOS DeviceSupport")},
		{"tvOS DeviceSupport", filepath.Join(home, "Library", "Developer", "Xcode", "tvOS DeviceSupport")},
		{"watchOS DeviceSupport", filepath.Join(home, "Library", "Developer", "Xcode", "watchOS DeviceSupport")},
		{"Android AVD", filepath.Join(home, ".android", "avd")},
		{"Docker build cache", filepath.Join(home, "Library", "Containers", "com.docker.docker", "Data")},
		{"Composer cache", filepath.Join(home, ".composer", "cache")},
		{"Ruby gems cache", filepath.Join(home, ".gem")},
		{"Bundler cache", filepath.Join(home, ".bundle")},
	}

	var caches []Cache
	for _, c := range candidates {
		if _, err := os.Stat(c.path); err == nil {
			caches = append(caches, Cache{
				Name:      c.name,
				Path:      c.path,
				SizeBytes: 0, // computed lazily
			})
		}
	}
	return caches
}
