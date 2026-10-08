//go:build windows

package privatefs

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// EnsureDir creates dir and gives the current user full control over it and
// all files and directories created inside it, without requiring elevation.
func EnsureDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	dir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	tokenUser, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	trustee := windows.TRUSTEE{
		MultipleTrustee:          nil,
		MultipleTrusteeOperation: windows.NO_MULTIPLE_TRUSTEE,
		TrusteeForm:              windows.TRUSTEE_IS_SID,
		TrusteeType:              windows.TRUSTEE_IS_USER,
		TrusteeValue:             windows.TrusteeValueFromSID(tokenUser.User.Sid),
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.SET_ACCESS,
		Inheritance:       windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
		Trustee:           trustee,
	}}, nil)
	if err != nil {
		return err
	}
	const flags = windows.DACL_SECURITY_INFORMATION | windows.PROTECTED_DACL_SECURITY_INFORMATION
	return windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, flags, nil, nil, acl, nil)
}
