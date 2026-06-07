package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type SandboxOrganization struct {
	Id             int            `json:"id"`
	OrganizationID string         `json:"organization_id" gorm:"size:64;not null;uniqueIndex"`
	CreatedTime    int64          `json:"created_time" gorm:"bigint;index"`
	UpdatedTime    int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

type SandboxOrgToken struct {
	Id                    int            `json:"id"`
	SandboxOrganizationId int            `json:"sandbox_organization_id" gorm:"not null;index"`
	TokenId               int            `json:"token_id" gorm:"not null;uniqueIndex"`
	CreatedTime           int64          `json:"created_time" gorm:"bigint;index"`
	DeletedAt             gorm.DeletedAt `json:"-" gorm:"index"`
}

func GetSandboxOrganizationByOrganizationID(organizationID string) (*SandboxOrganization, error) {
	if organizationID == "" {
		return nil, errors.New("organization_id 不能为空")
	}
	var organization SandboxOrganization
	if err := DB.Where("organization_id = ?", organizationID).First(&organization).Error; err != nil {
		return nil, err
	}
	return &organization, nil
}

func CreateOrGetSandboxOrganization(organizationID string) (*SandboxOrganization, bool, error) {
	if organizationID == "" {
		return nil, false, errors.New("organization_id 不能为空")
	}
	organization, err := GetSandboxOrganizationByOrganizationID(organizationID)
	if err == nil {
		return organization, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	now := common.GetTimestamp()
	organization = &SandboxOrganization{
		OrganizationID: organizationID,
		CreatedTime:    now,
		UpdatedTime:    now,
	}
	if err := DB.Create(organization).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			existing, getErr := GetSandboxOrganizationByOrganizationID(organizationID)
			return existing, false, getErr
		}
		// Fall back to a second read for databases/drivers that do not surface duplicated-key uniformly.
		existing, getErr := GetSandboxOrganizationByOrganizationID(organizationID)
		if getErr == nil {
			return existing, false, nil
		}
		return nil, false, err
	}
	return organization, true, nil
}

func (relation *SandboxOrgToken) Insert() error {
	relation.CreatedTime = common.GetTimestamp()
	return DB.Create(relation).Error
}

func GetSandboxOrgTokenByTokenID(tokenID int) (*SandboxOrgToken, error) {
	if tokenID == 0 {
		return nil, errors.New("token_id 不能为空")
	}
	var relation SandboxOrgToken
	if err := DB.Where("token_id = ?", tokenID).First(&relation).Error; err != nil {
		return nil, err
	}
	return &relation, nil
}

func IsSandboxToken(tokenID int) (bool, error) {
	_, err := GetSandboxOrgTokenByTokenID(tokenID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}
