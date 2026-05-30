package repository

import (
	"strings"

	dbaccount "github.com/Wei-Shaw/sub2api/ent/account"
	dbpredicate "github.com/Wei-Shaw/sub2api/ent/predicate"
)

type accountErrorBucket string

const (
	accountErrorBucketNetwork     accountErrorBucket = "network"
	accountErrorBucketAuthInvalid accountErrorBucket = "auth_invalid"
	accountErrorBucketCF          accountErrorBucket = "cf"
	accountErrorBucketOther       accountErrorBucket = "other"
)

var accountErrorNetworkKeywords = []string{
	"eof",
	"connection reset",
	"connection refused",
	"timeout",
	"i/o timeout",
	"proxyconnect",
	"no such host",
	"tls handshake",
	"temporary failure in name resolution",
	"network is unreachable",
	"context deadline exceeded",
	"dial tcp",
}

var accountErrorAuthInvalidKeywords = []string{
	"token_invalidated",
	"authentication token has been invalidated",
	"invalidated",
	"token revoked",
	"revoked",
	"refresh token expired",
	"token expired",
	"expired",
	"unauthorized",
	"invalid_grant",
	"account_disabled_auth_error",
	"invalid refresh token",
	"401",
}

var accountErrorCFKeywords = []string{
	"cloudflare",
	"cf challenge",
	"cf-ray",
	"access forbidden",
	"temporary cooldown",
	"challenge",
	"403",
}

func accountErrorNetworkPredicate() dbpredicate.Account {
	return accountErrorKeywordPredicate(accountErrorNetworkKeywords)
}

func accountErrorAuthInvalidPredicate() dbpredicate.Account {
	return accountErrorKeywordPredicate(accountErrorAuthInvalidKeywords)
}

func accountErrorCFPredicate() dbpredicate.Account {
	return accountErrorKeywordPredicate(accountErrorCFKeywords)
}

func accountErrorKnownPredicate() dbpredicate.Account {
	return dbaccount.Or(
		accountErrorNetworkPredicate(),
		accountErrorAuthInvalidPredicate(),
		accountErrorCFPredicate(),
	)
}

func accountErrorKeywordPredicate(keywords []string) dbpredicate.Account {
	predicates := make([]dbpredicate.Account, 0, len(keywords))
	for _, keyword := range keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		predicates = append(predicates, dbaccount.ErrorMessageContainsFold(keyword))
	}
	if len(predicates) == 0 {
		return dbaccount.ErrorMessageEQ("__sub2api_no_account_error_keyword__")
	}
	return dbaccount.Or(predicates...)
}

func classifyAccountErrorMessage(message string) accountErrorBucket {
	lower := strings.ToLower(strings.TrimSpace(message))
	if containsAnyAccountErrorKeyword(lower, accountErrorAuthInvalidKeywords) {
		return accountErrorBucketAuthInvalid
	}
	if containsAnyAccountErrorKeyword(lower, accountErrorNetworkKeywords) {
		return accountErrorBucketNetwork
	}
	if containsAnyAccountErrorKeyword(lower, accountErrorCFKeywords) {
		return accountErrorBucketCF
	}
	return accountErrorBucketOther
}

func containsAnyAccountErrorKeyword(lowerMessage string, keywords []string) bool {
	for _, keyword := range keywords {
		keyword = strings.ToLower(strings.TrimSpace(keyword))
		if keyword != "" && strings.Contains(lowerMessage, keyword) {
			return true
		}
	}
	return false
}
