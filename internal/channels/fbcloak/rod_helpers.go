//go:build !sqliteonly

package fbcloak

import "github.com/go-rod/rod/lib/proto"

// rodTargetBlank returns the proto type for opening a fresh blank tab. Kept in
// a helper so test code does not need to import proto.
func rodTargetBlank() proto.TargetCreateTarget {
	return proto.TargetCreateTarget{URL: "about:blank"}
}
