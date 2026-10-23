package service

import (
	"crypto/hmac"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type ldapCandidatePayload struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Department  string `json:"department"`
	LDAPId      string `json:"ldap_id"`
}

func SearchLDAPUsers(identifier string) ([]LDAPSyncCandidate, error) {
	profiles, err := searchLDAPProfiles(identifier)
	if err != nil {
		return nil, err
	}
	result := make([]LDAPSyncCandidate, 0, len(profiles))
	for _, profile := range profiles {
		candidate := LDAPSyncCandidate{
			Key: profile.LDAPId, Username: profile.Username, Email: profile.Email,
			DisplayName: ldapDisplayName(profile), Department: profile.Department, LDAPId: profile.LDAPId,
		}
		candidate.Signature = signLDAPSyncCandidate(candidate)
		result = append(result, candidate)
	}
	return result, nil
}

func SyncLDAPUser(identifier string) (*model.User, error) {
	profiles, err := searchLDAPProfiles(identifier)
	if err != nil {
		return nil, err
	}
	if len(profiles) != 1 {
		return nil, errors.New("ldap identity is ambiguous")
	}
	user, _, err := findOrCreateLDAPUser(profiles[0], true)
	return user, err
}

func SyncLDAPCandidate(candidate LDAPSyncCandidate) (*model.User, error) {
	if _, err := buildLDAPConfig(); err != nil {
		return nil, err
	}
	if !verifyLDAPSyncCandidate(candidate) {
		return nil, ErrLDAPCandidateInvalid
	}
	profile, err := normalizeLDAPProfile(ldapProfile{
		Username: candidate.Username, Email: candidate.Email,
		DisplayName: candidate.DisplayName, Department: candidate.Department, LDAPId: candidate.LDAPId,
	})
	if err != nil {
		return nil, err
	}
	user, _, err := findOrCreateLDAPUser(profile, true)
	return user, err
}

func searchLDAPProfiles(identifier string) ([]ldapProfile, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, errors.New("ldap identifier is empty")
	}
	config, err := buildLDAPConfig()
	if err != nil {
		return nil, err
	}
	users := ldapSearch(config, identifier)
	if len(users) == 0 && strings.Contains(identifier, "@") && !isLDAPEmailAttribute(config.LdapUID) {
		config.LdapUID = "mail"
		users = ldapSearch(config, identifier)
	}
	if len(users) == 0 {
		return nil, errors.New("ldap identity not found")
	}
	profiles := make([]ldapProfile, 0, len(users))
	seen := map[string]struct{}{}
	for _, user := range users {
		profile, err := profileFromLDAPUser(user, identifier)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[profile.LDAPId]; ok {
			continue
		}
		seen[profile.LDAPId] = struct{}{}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

func isLDAPEmailAttribute(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "mail" || value == "email"
}

func signLDAPSyncCandidate(candidate LDAPSyncCandidate) string {
	payload := ldapCandidatePayload{
		Username: strings.TrimSpace(candidate.Username), Email: model.NormalizeEmail(candidate.Email),
		DisplayName: strings.TrimSpace(candidate.DisplayName), Department: strings.TrimSpace(candidate.Department), LDAPId: strings.TrimSpace(candidate.LDAPId),
	}
	data, err := common.Marshal(payload)
	if err != nil {
		return ""
	}
	return common.GenerateHMAC(string(data))
}

func verifyLDAPSyncCandidate(candidate LDAPSyncCandidate) bool {
	if strings.TrimSpace(candidate.Signature) == "" {
		return false
	}
	expected := signLDAPSyncCandidate(candidate)
	return hmac.Equal([]byte(candidate.Signature), []byte(expected))
}
