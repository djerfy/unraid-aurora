package provider

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"aurora/internal/model"
)

var userNamePattern = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,39}$`)

func validateUserName(name string) error {
	if !userNamePattern.MatchString(name) {
		return errors.New("user name must start with a lowercase letter or underscore and contain only lowercase letters, digits, '_' or '-'")
	}
	return nil
}

func validateUserDescription(description string) error {
	if len(description) > 64 {
		return errors.New("description must be 64 characters or fewer")
	}
	for _, char := range description {
		if char == '&' || char == ':' || char == '"' || unicode.IsControl(char) {
			return errors.New(`description cannot contain &, :, " or control characters`)
		}
	}
	return nil
}

func validateUserPassword(password string) error {
	if password == "" {
		return errors.New("a password is required")
	}
	if len(password) > 128 {
		return errors.New("password must be 128 characters or fewer")
	}
	if strings.ContainsRune(password, 0) {
		return errors.New("password contains an invalid character")
	}
	return nil
}

func encodeUserPassword(password string) string {
	return base64.StdEncoding.EncodeToString([]byte(password))
}

// CreateUser mirrors the Add User page of the official webGUI. emhttpd creates
// the Unix account, the Samba account and the share access entry.
func (l *Live) CreateUser(ctx context.Context, opts model.UserCreateOptions) error {
	if err := validateUserName(opts.Name); err != nil {
		return err
	}
	if err := validateUserDescription(opts.Description); err != nil {
		return err
	}
	if err := validateUserPassword(opts.Password); err != nil {
		return err
	}
	values := url.Values{
		"cmdUserEdit":      {"Add"},
		"userName":         {opts.Name},
		"userDesc":         {opts.Description},
		"userPassword":     {encodeUserPassword(opts.Password)},
		"userPasswordConf": {encodeUserPassword(opts.Password)},
	}
	return runEmcmd(ctx, values.Encode())
}

func (l *Live) UpdateUser(ctx context.Context, name, description string) error {
	if err := validateUserName(name); err != nil {
		return err
	}
	if err := validateUserDescription(description); err != nil {
		return err
	}
	values := url.Values{
		"cmdUserEdit": {"Apply"},
		"userName":    {name},
		"userDesc":    {description},
	}
	return runEmcmd(ctx, values.Encode())
}

func (l *Live) ChangeUserPassword(ctx context.Context, name, password string) error {
	if err := validateUserName(name); err != nil {
		return err
	}
	if err := validateUserPassword(password); err != nil {
		return err
	}
	values := url.Values{
		"cmdUserEdit":      {"Change"},
		"userName":         {name},
		"userPassword":     {encodeUserPassword(password)},
		"userPasswordConf": {encodeUserPassword(password)},
	}
	return runEmcmd(ctx, values.Encode())
}

func (l *Live) DeleteUser(ctx context.Context, name string) error {
	if name == "root" {
		return errors.New("the root account cannot be deleted")
	}
	if err := validateUserName(name); err != nil {
		return err
	}
	values := url.Values{
		"cmdUserEdit":   {"Delete"},
		"userName":      {name},
		"confirmDelete": {"on"},
	}
	if err := runEmcmd(ctx, values.Encode()); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}
