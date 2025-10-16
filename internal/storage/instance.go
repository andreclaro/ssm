package storage

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	ectype "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// InstanceFilter represents filters for instance queries
type InstanceFilter struct {
	Profile *string
	Region  *string
	Name    *string
	State   *string
}

// InstanceRepository handles database operations for instances
type InstanceRepository struct{}

// NewInstanceRepository creates a new instance repository
func NewInstanceRepository() *InstanceRepository {
	return &InstanceRepository{}
}

// SaveOrUpdate saves or updates an instance in the database
func (r *InstanceRepository) SaveOrUpdate(instance *Instance) error {
	// Use transaction to ensure data consistency
	return DB.Transaction(func(tx *gorm.DB) error {
		// Upsert instance
		if err := tx.Where(Instance{
			InstanceID: instance.InstanceID,
			Region:     instance.Region,
			Profile:    instance.Profile,
		}).Assign(Instance{
			Name:      instance.Name,
			AccountID: instance.AccountID,
			State:     instance.State,
			Platform:  instance.Platform,
			LastSeen:  time.Now(),
		}).FirstOrCreate(instance).Error; err != nil {
			return fmt.Errorf("failed to save instance: %w", err)
		}

		// Only replace tags if provided to avoid wiping tags on partial updates (e.g., SSM sync)
		if len(instance.Tags) > 0 {
			// Delete existing tags for this instance
			if err := tx.Where("instance_id = ?", instance.InstanceID).Delete(&Tag{}).Error; err != nil {
				return fmt.Errorf("failed to delete existing tags: %w", err)
			}

			// Insert new tags in batches to reduce round-trips
			newTags := make([]Tag, 0, len(instance.Tags))
			for _, tag := range instance.Tags {
				tag.InstanceID = instance.InstanceID
				newTags = append(newTags, tag)
			}
			if len(newTags) > 0 {
				if err := tx.CreateInBatches(newTags, 100).Error; err != nil {
					return fmt.Errorf("failed to save tags: %w", err)
				}
			}
		}

		return nil
	})
}

// SaveOrUpdateBatch saves or updates multiple instances within a single transaction
func (r *InstanceRepository) SaveOrUpdateBatch(instances []*Instance) error {
	if len(instances) == 0 {
		return nil
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now()

		for _, instance := range instances {
			// Upsert instance
			if err := tx.Where(Instance{
				InstanceID: instance.InstanceID,
				Region:     instance.Region,
				Profile:    instance.Profile,
			}).Assign(Instance{
				Name:      instance.Name,
				AccountID: instance.AccountID,
				State:     instance.State,
				Platform:  instance.Platform,
				LastSeen:  now,
			}).FirstOrCreate(instance).Error; err != nil {
				return fmt.Errorf("failed to save instance: %w", err)
			}

			// Only replace tags if provided to avoid wiping tags on partial updates
			if len(instance.Tags) > 0 {
				if err := tx.Where("instance_id = ?", instance.InstanceID).Delete(&Tag{}).Error; err != nil {
					return fmt.Errorf("failed to delete existing tags: %w", err)
				}

				newTags := make([]Tag, 0, len(instance.Tags))
				for _, tag := range instance.Tags {
					tag.InstanceID = instance.InstanceID
					newTags = append(newTags, tag)
				}
				if len(newTags) > 0 {
					if err := tx.CreateInBatches(newTags, 100).Error; err != nil {
						return fmt.Errorf("failed to save tags: %w", err)
					}
				}
			}
		}

		return nil
	})
}

// FindByName finds an instance by name, preferring reachable instances.
// Preference order:
//  1. SSM Online
//  2. EC2 running
//  3. Everything else (e.g., ConnectionLost, stopped)
//
// Within the same priority, choose the most recently seen/updated.
func (r *InstanceRepository) FindByName(name string) (*Instance, error) {
	var instance Instance
	// Use CASE ordering to prioritize desired states, then favor newest records.
	// Note: "running" may be stored in various cases, so compare in lower().
	orderExpr := `CASE 
        WHEN state = 'Online' THEN 0 
        WHEN lower(state) = 'running' THEN 1 
        ELSE 2 
    END ASC, last_seen DESC, updated_at DESC`

	if err := DB.Preload("Tags").Where("name = ?", name).Order(orderExpr).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Try again by stripping common domain suffixes (e.g., .maas)
			// This allows connecting with either base name or FQDN.
			var alt Instance
			if idx := indexOfDot(name); idx > 0 {
				base := name[:idx]
				if err2 := DB.Preload("Tags").Where("name = ?", base).Order(orderExpr).First(&alt).Error; err2 == nil {
					return &alt, nil
				}
			}
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find instance by name: %w", err)
	}
	return &instance, nil
}

// indexOfDot returns the index of the first '.' in s, or -1 if none
func indexOfDot(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return i
		}
	}
	return -1
}

// FindByID finds an instance by instance ID
func (r *InstanceRepository) FindByID(instanceID string) (*Instance, error) {
	var instance Instance
	if err := DB.Preload("Tags").Where("instance_id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find instance by ID: %w", err)
	}
	return &instance, nil
}

// List returns a list of instances with optional filters
func (r *InstanceRepository) List(filter *InstanceFilter) ([]Instance, error) {
	var instances []Instance
	query := DB

	if filter != nil {
		if filter.Profile != nil {
			query = query.Where("profile = ?", *filter.Profile)
		}
		if filter.Region != nil {
			query = query.Where("region = ?", *filter.Region)
		}
		if filter.Name != nil {
			query = query.Where("name LIKE ?", "%"+*filter.Name+"%")
		}
		if filter.State != nil {
			query = query.Where("state = ?", *filter.State)
		}
	}

	// Order alphabetically by profile (account), region, then name for stable listing
	query = query.Order("profile ASC").Order("region ASC").Order("name ASC")

	if err := query.Find(&instances).Error; err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	return instances, nil
}

// DeleteStale removes instances that haven't been seen for more than the specified duration
func (r *InstanceRepository) DeleteStale(olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	result := DB.Where("last_seen < ?", cutoff).Delete(&Instance{})

	if result.Error != nil {
		return fmt.Errorf("failed to delete stale instances: %w", result.Error)
	}

	if result.RowsAffected > 0 {
		logrus.WithField("count", result.RowsAffected).Info("Deleted stale instances")
	}

	return nil
}

// GetStats returns statistics about stored instances
func (r *InstanceRepository) GetStats() (map[string]int, error) {
	stats := make(map[string]int)

	// Total instances
	var total int64
	if err := DB.Model(&Instance{}).Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count instances: %w", err)
	}
	stats["total"] = int(total)

	// Instances by profile
	var profiles []struct {
		Profile string
		Count   int
	}
	if err := DB.Model(&Instance{}).Select("profile, count(*) as count").Group("profile").Find(&profiles).Error; err != nil {
		return nil, fmt.Errorf("failed to count instances by profile: %w", err)
	}
	for _, p := range profiles {
		stats["profile_"+p.Profile] = p.Count
	}

	// Instances by region
	var regions []struct {
		Region string
		Count  int
	}
	if err := DB.Model(&Instance{}).Select("region, count(*) as count").Group("region").Find(&regions).Error; err != nil {
		return nil, fmt.Errorf("failed to count instances by region: %w", err)
	}
	for _, r := range regions {
		stats["region_"+r.Region] = r.Count
	}

	return stats, nil
}

// ConvertEC2Instance converts an EC2 instance to our Instance model
func ConvertEC2Instance(ec2Instance ectype.Instance, region, profile, accountID string) *Instance {
	instance := &Instance{
		InstanceID: *ec2Instance.InstanceId,
		Region:     region,
		Profile:    profile,
		AccountID:  accountID,
		State:      string(ec2Instance.State.Name),
	}

	// Extract name from tags
	if ec2Instance.Tags != nil {
		tags := make([]Tag, 0, len(ec2Instance.Tags))
		for _, tag := range ec2Instance.Tags {
			if tag.Key != nil && tag.Value != nil {
				if *tag.Key == "Name" {
					instance.Name = *tag.Value
				}
				tags = append(tags, Tag{
					Key:   *tag.Key,
					Value: *tag.Value,
				})
			}
		}
		instance.Tags = tags
	}

	// Set platform
	if ec2Instance.PlatformDetails != nil {
		instance.Platform = *ec2Instance.PlatformDetails
	}

	return instance
}

// ConvertSSMManagedInstance converts SSM managed instance info to our Instance model
func ConvertSSMManagedInstance(info ssmtypes.InstanceInformation, region, profile, accountID string) *Instance {
	instance := &Instance{
		InstanceID: *info.InstanceId,
		Region:     region,
		Profile:    profile,
		AccountID:  accountID,
		State:      string(info.PingStatus),
	}

	// Prefer SSM Name; fall back to ComputerName if Name is empty
	if info.Name != nil && *info.Name != "" {
		instance.Name = *info.Name
	} else if info.ComputerName != nil {
		instance.Name = *info.ComputerName
	}
	if info.PlatformName != nil {
		instance.Platform = *info.PlatformName
	}

	// SSM DescribeInstanceInformation does not return EC2 tags; skip tags here
	return instance
}
