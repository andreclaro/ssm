package storage

import (
	"fmt"
	"strings"

	"github.com/andreclaro/ssm/internal/aws"
)

// ProfileRepository handles database operations for profiles
type ProfileRepository struct{}

// NewProfileRepository creates a new profile repository
func NewProfileRepository() *ProfileRepository {
	return &ProfileRepository{}
}

// GetEnabledProfiles returns all enabled profiles
func (r *ProfileRepository) GetEnabledProfiles() ([]string, error) {
	var profiles []Profile
	if err := DB.Where("enabled = ?", true).Find(&profiles).Error; err != nil {
		return nil, fmt.Errorf("failed to get enabled profiles: %w", err)
	}

	profileNames := make([]string, len(profiles))
	for i, profile := range profiles {
		profileNames[i] = profile.Profile
	}

	return profileNames, nil
}

// GetAllProfiles returns all profiles (enabled and disabled)
func (r *ProfileRepository) GetAllProfiles() ([]Profile, error) {
	var profiles []Profile
	if err := DB.Order("profile").Find(&profiles).Error; err != nil {
		return nil, fmt.Errorf("failed to get all profiles: %w", err)
	}

	return profiles, nil
}

// EnableProfile enables a profile for discovery
func (r *ProfileRepository) EnableProfile(profileName string) error {
	return DB.Where(Profile{Profile: profileName}).Assign(Profile{Enabled: true}).FirstOrCreate(&Profile{}).Error
}

// DisableProfile disables a profile for discovery
func (r *ProfileRepository) DisableProfile(profileName string) error {
	return DB.Model(&Profile{}).Where("profile = ?", profileName).Update("enabled", false).Error
}

// InitializeProfiles discovers and initializes all available AWS profiles
func (r *ProfileRepository) InitializeProfiles() error {
	// Check if profiles are already initialized
	var count int64
	if err := DB.Model(&Profile{}).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to count profiles: %w", err)
	}

	if count > 0 {
		// Profiles already initialized
		return nil
	}

	// Discover available profiles
	availableProfiles, err := aws.GetAvailableProfiles()
	if err != nil {
		return fmt.Errorf("failed to get available profiles: %w", err)
	}

	// Enable all discovered profiles by default
	for _, profileName := range availableProfiles {
		if err := r.EnableProfile(profileName); err != nil {
			return fmt.Errorf("failed to enable profile %s: %w", profileName, err)
		}
	}

	return nil
}

// SetProfiles enables only the specified profiles and disables others
func (r *ProfileRepository) SetProfiles(enabledProfiles []string) error {
	// Create a map for quick lookup
	enabledMap := make(map[string]bool)
	for _, profile := range enabledProfiles {
		enabledMap[profile] = true
	}

	// Get all existing profiles
	var allProfiles []Profile
	if err := DB.Find(&allProfiles).Error; err != nil {
		return fmt.Errorf("failed to get all profiles: %w", err)
	}

	// Update each profile
	for _, profile := range allProfiles {
		shouldEnable := enabledMap[profile.Profile]
		if profile.Enabled != shouldEnable {
			if err := DB.Model(&profile).Update("enabled", shouldEnable).Error; err != nil {
				return fmt.Errorf("failed to update profile %s: %w", profile.Profile, err)
			}
		}
	}

	// Enable any new profiles that don't exist yet
	for _, profileName := range enabledProfiles {
		if err := r.EnableProfile(profileName); err != nil {
			// Ignore error if profile already exists
			if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
				return fmt.Errorf("failed to enable profile %s: %w", profileName, err)
			}
		}
	}

	return nil
}
