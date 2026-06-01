package mail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const resendURL = "https://api.resend.com/emails"

// Client sends transactional emails via the Resend API.
type Client struct {
	apiKey    string
	fromEmail string
}

func NewClient(apiKey, fromEmail string) *Client {
	return &Client{apiKey: apiKey, fromEmail: fromEmail}
}

func (c *Client) send(to, subject, html string) error {
	from := fmt.Sprintf("Acareca <%s>", c.fromEmail)

	payload := map[string]interface{}{
		"from":    from,
		"to":      []string{to},
		"subject": subject,
		"html":    html,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, resendURL, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend error: %s", string(body))
	}
	return nil
}

// SendVerificationEmail sends an account verification link to the user.
func (c *Client) SendVerificationEmail(to, firstName, verificationLink string) error {
	html := fmt.Sprintf(`
<div
	style="font-family: sans-serif; color: #333; max-width: 600px; margin: auto; border: 1px solid #eee; padding: 20px;">
	<h2 style="color: #1a73e8;">Verify your email</h2>
	<p>Hi %s,</p>
	<p>Thank you for signing up with Acareca! To complete your registration, please verify your email address:</p>
	<div style="text-align: center; margin: 30px 0;">
		<a href="%s"
			style="background-color: #1a73e8; color: white; padding: 14px 28px; text-decoration: none; border-radius: 4px; font-weight: bold; display: inline-block;">
			Verify My Account
		</a>
	</div>
	<p style="font-size: 14px; color: #666;">If the button doesn't work, copy and paste this link into your browser:</p>
	<p style="font-size: 12px; word-break: break-all; color: #1a73e8;">%s</p>
	<p style="font-size: 14px; color: #666;">This link expires in <strong>10 minutes</strong>.</p>
	<hr style="border: none; border-top: 1px solid #eee; margin: 20px 0;" />
	<p style="font-size: 12px; color: #888;">If you did not create this account, you can safely ignore this email.</p>
	<p style="font-size: 12px; color: #888;">Best regards,<br>The Acareca Team</p>
</div>`, firstName, verificationLink, verificationLink)

	return c.send(to, "Verify your Acareca account", html)
}

// SendPasswordResetEmail sends a password reset link to the user.
func (c *Client) SendPasswordResetEmail(to, firstName, resetLink string) error {
	html := fmt.Sprintf(`
<div
	style="font-family: sans-serif; color: #333; max-width: 600px; margin: auto; border: 1px solid #eee; padding: 20px;">
	<h2 style="color: #1a73e8;">Password Reset Request</h2>
	<p>Hi %s,</p>
	<p>We received a request to reset your Acareca password. Click below to choose a new one:</p>
	<div style="text-align: center; margin: 30px 0;">
		<a href="%s"
			style="background-color: #1a73e8; color: white; padding: 14px 28px; text-decoration: none; border-radius: 4px; font-weight: bold; display: inline-block;">
			Reset My Password
		</a>
	</div>
	<p style="font-size: 14px; color: #666;">If the button doesn't work, copy and paste this link into your browser:</p>
	<p style="font-size: 12px; word-break: break-all; color: #1a73e8;">%s</p>
	<p style="font-size: 14px; color: #666;">This link expires in <strong>15 minutes</strong>.</p>
	<hr style="border: none; border-top: 1px solid #eee; margin: 20px 0;" />
	<p style="font-size: 12px; color: #888;">If you did not request this, your password will remain unchanged.</p>
	<p style="font-size: 12px; color: #888;">Best regards,<br>The Acareca Team</p>
</div>`, firstName, resetLink, resetLink)

	return c.send(to, "Reset your Acareca password", html)
}

// SendInvitationEmail sends an invite to an accountant.
func (c *Client) SendInvitationEmail(to, senderName, inviteLink string) error {
	namePart := strings.Split(to, "@")[0]
	namePart = strings.ReplaceAll(namePart, ".", " ")
	namePart = strings.ReplaceAll(namePart, "_", " ")
	recipientName := cases.Title(language.English).String(namePart)

	html := fmt.Sprintf(`
<div style="font-family: sans-serif; color: #333; line-height: 1.6;">
	<p style="font-size: 14px;">Hello <strong>%s</strong>,</p>
	<p><strong>%s</strong> has invited you to collaborate on <strong>Acareca</strong> as their Accountant/Bookkeeper.
	</p>
	<p>Acareca is a secure platform designed to streamline financial management and document sharing between
		practitioners and financial professionals.</p>
	<div style="margin: 30px 0;">
		<a href="%s"
			style="background-color: #1a73e8; color: white; padding: 12px 24px; text-decoration: none; border-radius: 4px; font-weight: bold;">
			Access Client Files
		</a>
	</div>
	<p style="font-size: 14px; color: #666;">If the button doesn't work, copy and paste this link into your browser:</p>
	<p style="font-size: 12px; word-break: break-all; color: #1a73e8;">%s</p>
	<p style="margin-top: 30px">By accepting this invitation, you will be able to view and manage financial records shared by %s.</p>
	<hr style="border: none; border-top: 1px solid #eee; margin: 20px 0;" />
	<small style="color: #888;">This invitation was intended for %s and will expire in 7 days.</small>
</div>`, recipientName, senderName, inviteLink, inviteLink, senderName, to)

	subject := fmt.Sprintf("Invitation: Manage %s's files on Acareca", senderName)
	return c.send(to, subject, html)
}
