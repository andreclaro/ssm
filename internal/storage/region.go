package storage

import (
	"fmt"
)

// RegionRepository handles database operations for regions
type RegionRepository struct{}

// NewRegionRepository creates a new region repository
func NewRegionRepository() *RegionRepository {
	return &RegionRepository{}
}

// GetEnabledRegions returns all enabled regions
func (r *RegionRepository) GetEnabledRegions() ([]string, error) {
	var regions []Region
	if err := DB.Where("enabled = ?", true).Find(&regions).Error; err != nil {
		return nil, fmt.Errorf("failed to get enabled regions: %w", err)
	}

	regionNames := make([]string, len(regions))
	for i, region := range regions {
		regionNames[i] = region.Region
	}

	return regionNames, nil
}

// GetAllRegions returns all regions (enabled and disabled)
func (r *RegionRepository) GetAllRegions() ([]Region, error) {
	var regions []Region
	if err := DB.Order("region").Find(&regions).Error; err != nil {
		return nil, fmt.Errorf("failed to get all regions: %w", err)
	}

	return regions, nil
}

// EnableRegion enables a region for discovery
func (r *RegionRepository) EnableRegion(regionName string) error {
	return DB.Where(Region{Region: regionName}).Assign(Region{Enabled: true}).FirstOrCreate(&Region{}).Error
}

// DisableRegion disables a region for discovery
func (r *RegionRepository) DisableRegion(regionName string) error {
	return DB.Model(&Region{}).Where("region = ?", regionName).Update("enabled", false).Error
}

// SetDefaultRegions sets up the default regions (common ones enabled by default)
func (r *RegionRepository) SetDefaultRegions() error {
	defaultRegions := []string{
		"us-east-1", "us-east-2", "us-west-1", "us-west-2",
		"eu-west-1", "eu-central-1",
		"ap-southeast-1", "ap-southeast-2",
		"ca-central-1", "sa-east-1",
	}

	for _, region := range defaultRegions {
		if err := r.EnableRegion(region); err != nil {
			return fmt.Errorf("failed to enable default region %s: %w", region, err)
		}
	}

	return nil
}

// InitializeRegions ensures that regions are initialized with defaults if empty
func (r *RegionRepository) InitializeRegions() error {
	var count int64
	if err := DB.Model(&Region{}).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to count regions: %w", err)
	}

	if count == 0 {
		return r.SetDefaultRegions()
	}

	return nil
}
