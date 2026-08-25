package model

import (
	"strconv"
	"strings"
)

func SearchRelayAlertTokens(keyword string, limit int) ([]Token, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	query := DB.Model(&Token{}).Select("id", "user_id", "name", "status").Order("id desc").Limit(limit)
	if keyword != "" {
		pattern := relayAlertLikePattern(keyword)
		if id, err := strconv.Atoi(keyword); err == nil {
			query = query.Where("id = ? OR name LIKE ? ESCAPE '!'", id, pattern)
		} else {
			query = query.Where("name LIKE ? ESCAPE '!'", pattern)
		}
	}
	var tokens []Token
	return tokens, query.Find(&tokens).Error
}

func SearchRelayAlertUsers(keyword string, limit int) ([]User, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	query := DB.Model(&User{}).Select("id", "username", "display_name", "email", "status").Order("id desc").Limit(limit)
	if keyword != "" {
		pattern := relayAlertLikePattern(keyword)
		if id, err := strconv.Atoi(keyword); err == nil {
			query = query.Where("id = ? OR username LIKE ? ESCAPE '!' OR display_name LIKE ? ESCAPE '!' OR email LIKE ? ESCAPE '!'", id, pattern, pattern, pattern)
		} else {
			query = query.Where("username LIKE ? ESCAPE '!' OR display_name LIKE ? ESCAPE '!' OR email LIKE ? ESCAPE '!'", pattern, pattern, pattern)
		}
	}
	var users []User
	return users, query.Find(&users).Error
}

func SearchRelayAlertChannels(keyword string, limit int) ([]Channel, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	query := DB.Model(&Channel{}).Select("id", "name", "type", "status").Order("id desc").Limit(limit)
	if keyword != "" {
		pattern := relayAlertLikePattern(keyword)
		if id, err := strconv.Atoi(keyword); err == nil {
			query = query.Where("id = ? OR name LIKE ? ESCAPE '!'", id, pattern)
		} else {
			query = query.Where("name LIKE ? ESCAPE '!'", pattern)
		}
	}
	var channels []Channel
	return channels, query.Find(&channels).Error
}

func relayAlertLikePattern(keyword string) string {
	keyword = strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(strings.TrimSpace(keyword))
	return "%" + keyword + "%"
}
