package crypto

import (
	"fmt"
	"path"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	mapperFilePathPrefix = "/dev/mapper"
)

// VolumeMapper returns the path for mapped encrypted device.
func VolumeMapper(volume string) string {
	return path.Join(mapperFilePathPrefix, volume)
}

// EncryptVolume encrypts provided device with LUKS.
func EncryptVolume(devicePath, passphrase string) error {
	logrus.Debugf("Encrypting device %s with LUKS", devicePath)
	if _, _, err := luksFormat(devicePath, passphrase); err != nil {
		return fmt.Errorf("failed to encrypt device %s with LUKS: %w", devicePath, err)
	}
	return nil
}

// OpenVolume opens volume so that it can be used by the client.
func OpenVolume(volume, devicePath, passphrase string) error {
	if isOpen, _ := IsDeviceOpen(VolumeMapper(volume)); isOpen {
		logrus.Debugf("device %s is already opened at %s", devicePath, VolumeMapper(volume))
		return nil
	}

	logrus.Debugf("Opening device %s with LUKS on %s", devicePath, volume)
	_, stderr, err := luksOpen(volume, devicePath, passphrase)
	if err != nil {
		logrus.Warnf("failed to open LUKS device %s: %s", devicePath, stderr)
	}
	return err
}

// CloseVolume closes encrypted volume so it can be detached.
func CloseVolume(volume string) error {
	logrus.Debugf("Closing LUKS device %s", volume)
	_, _, err := luksClose(volume)
	return err
}

// IsDeviceOpen determines if encrypted device is already open.
func IsDeviceOpen(device string) (bool, error) {
	_, mappedFile, err := DeviceEncryptionStatus(device)
	return mappedFile != "", err
}

// DeviceEncryptionStatus looks to identify if the passed device is a LUKS mapping
// and if so what the device is and the mapper name as used by LUKS.
// If not, just returns the original device and an empty string.
func DeviceEncryptionStatus(devicePath string) (mappedDevice, mapper string, err error) {
	if !strings.HasPrefix(devicePath, mapperFilePathPrefix) {
		return devicePath, "", nil
	}
	volume := strings.TrimPrefix(devicePath, mapperFilePathPrefix+"/")
	stdout, _, err := luksStatus(volume)
	if err != nil {
		logrus.Debugf("device %s is not an active LUKS device: %v", devicePath, err)
		return devicePath, "", nil
	}
	lines := strings.Split(string(stdout), "\n")
	if len(lines) < 1 {
		return "", "", fmt.Errorf("device encryption status returned no stdout for %s", devicePath)
	}
	if !strings.HasSuffix(lines[0], " is active.") {
		// Implies this is not a LUKS device
		return devicePath, "", nil
	}
	for i := 1; i < len(lines); i++ {
		kv := strings.SplitN(strings.TrimSpace(lines[i]), ":", 2)
		if len(kv) < 1 {
			return "", "", fmt.Errorf("device encryption status output for %s is badly formatted: %s",
				devicePath, lines[i])
		}
		if strings.Compare(kv[0], "device") == 0 {
			return strings.TrimSpace(kv[1]), volume, nil
		}
	}
	// Identified as LUKS, but failed to identify a mapped device
	return "", "", fmt.Errorf("mapped device not found in path %s", devicePath)
}
