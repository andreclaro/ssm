package storage

import (
	"time"

	"gorm.io/gorm"
)

// Instance represents an EC2 instance in the database
type Instance struct {
	ID         uint      `gorm:"primarykey" json:"-"`
	InstanceID string    `gorm:"uniqueIndex:idx_instance_profile_region;size:20" json:"instance_id"`
	Name       string    `gorm:"index;size:255" json:"name"`
	Region     string    `gorm:"uniqueIndex:idx_instance_profile_region;size:20" json:"region"`
	Profile    string    `gorm:"uniqueIndex:idx_instance_profile_region;size:100" json:"profile"`
	AccountID  string    `gorm:"index;size:20" json:"account_id"`
	State      string    `gorm:"size:20" json:"state"`
	Platform   string    `gorm:"size:50" json:"platform"`
	LastSeen   time.Time `json:"last_seen"`
	CreatedAt  time.Time `json:"-"`
	UpdatedAt  time.Time `json:"-"`

	Tags []Tag `gorm:"foreignKey:InstanceID;references:InstanceID" json:"tags"`
}

// Tag represents an EC2 instance tag
type Tag struct {
	ID         uint   `gorm:"primarykey" json:"-"`
	InstanceID string `gorm:"index;size:20" json:"instance_id"`
	Key        string `gorm:"size:128" json:"key"`
	Value      string `gorm:"size:256" json:"value"`
}

// Region represents a user-selected AWS region for discovery
type Region struct {
	ID      uint   `gorm:"primarykey" json:"-"`
	Region  string `gorm:"uniqueIndex;size:20" json:"region"`
	Enabled bool   `gorm:"default:true" json:"enabled"`
}

// Profile represents a user-selected AWS profile for discovery
type Profile struct {
	ID      uint   `gorm:"primarykey" json:"-"`
	Profile string `gorm:"uniqueIndex;size:100" json:"profile"`
	Enabled bool   `gorm:"default:true" json:"enabled"`
}

// TableName specifies the table name for Instance
func (Instance) TableName() string {
	return "instances"
}

// TableName specifies the table name for Tag
func (Tag) TableName() string {
	return "tags"
}

// TableName specifies the table name for Region
func (Region) TableName() string {
	return "regions"
}

// TableName specifies the table name for Profile
func (Profile) TableName() string {
	return "profiles"
}

// BeforeCreate sets the LastSeen timestamp before creating a record
func (i *Instance) BeforeCreate(tx *gorm.DB) error {
	i.LastSeen = time.Now()
	return nil
}

// BeforeUpdate sets the LastSeen timestamp before updating a record
func (i *Instance) BeforeUpdate(tx *gorm.DB) error {
	i.LastSeen = time.Now()
	return nil
}
