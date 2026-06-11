package mail

import (
	"strings"
)

// const (
// 	DefaultInvoicePaidSubject = "Payment Confirmed & Receipt: Invoice {invoice_number}"
// 	DefaultInvoicePaidBody    = `
// <div style="font-family: sans-serif; color: #333; max-width: 600px; margin: auto; border: 1px solid #eee; padding: 20px;">
// 	<h2 style="color: #1492A5;">Payment Confirmed</h2>
// 	<p>Hi {contact_name},</p>
// 	<p>Thank you! Your payment for invoice <strong>{invoice_number}</strong> has been successfully received and processed.</p>
// 	<p>The invoice has been marked as <strong>PAID</strong> in full. We have attached an official receipt copy to this email as a PDF document for your records.</p>
// 	<hr style="border: none; border-top: 1px solid #eee; margin: 20px 0;" />
// 	<p style="font-size: 12px; color: #888;">Best regards,<br>The Acareca Team</p>
// </div>`
// )

const (
	DefaultInvoicePaidSubject = "Receipt for Invoice {invoice_number} — Payment Confirmed"
	DefaultInvoicePaidBody    = `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Payment Confirmed</title>
</head>
<body style="margin: 0; padding: 0; background-color: #f8fafc; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; -webkit-font-smoothing: antialiased;">
    <table width="100%" border="0" cellspacing="0" cellpadding="0" style="background-color: #f8fafc; padding: 60px 20px;">
        <tr>
            <td align="center">
                <table width="100%" max-width="560" border="0" cellspacing="0" cellpadding="0" style="max-width: 560px; background-color: #ffffff; border: 1px solid #e2e8f0; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.05), 0 2px 4px -1px rgba(0, 0, 0, 0.03);">
                    
                    <tr>
                        <td height="4" style="background-color: #1492A5; line-height: 4px; font-size: 4px;">&nbsp;</td>
                    </tr>

                    <tr>
                        <td style="padding: 48px 40px;">
                            <h2 style="margin: 0 0 28px 0; color: #0f172a; font-size: 24px; font-weight: 600; letter-spacing: -0.02em;">Payment Received</h2>
                            
                            <p style="margin: 0 0 16px 0; color: #334155; font-size: 15px; line-height: 24px;">Hi {contact_name},</p>
                            
                            <p style="margin: 0 0 32px 0; color: #334155; font-size: 15px; line-height: 24px;">Thank you for your payment. This email confirms that invoice <strong>{invoice_number}</strong> has been fully settled and processed.</p>
                            
                            <table width="100%" border="0" cellspacing="0" cellpadding="0" style="border-top: 1px solid #f1f5f9; border-bottom: 1px solid #f1f5f9; padding: 20px 0; margin-bottom: 32px;">
                                <tr>
                                    <td valign="middle" style="line-height: 1;">
                                        <span style="display: block; color: #64748b; font-size: 12px; font-weight: 500; text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 6px;">Invoice Number</span>
                                        <span style="color: #0f172a; font-size: 15px; font-weight: 600;">{invoice_number}</span>
                                    </td>
                                    <td align="right" valign="middle" style="line-height: 1;">
                                        <span style="display: block; color: #64748b; font-size: 12px; font-weight: 500; text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 6px;">Payment Status</span>
                                        <table border="0" cellspacing="0" cellpadding="0" style="display: inline-block;">
                                            <tr>
                                                <td style="background-color: #e6f6f4; color: #107281; font-size: 12px; font-weight: 600; padding: 6px 12px; border-radius: 6px; text-transform: uppercase; letter-spacing: 0.02em; line-height: 1;">
                                                    Paid
                                                </td>
                                            </tr>
                                        </table>
                                    </td>
                                </tr>
                            </table>

                            <p style="margin: 0 0 32px 0; color: #475569; font-size: 14px; line-height: 22px;">A formal PDF receipt copy has been generated and attached below for your records.</p>
                            
                            <p style="margin: 0; color: #64748b; font-size: 14px; line-height: 22px;">Best regards,<br><span style="color: #0f172a; font-weight: 500; display: inline-block; margin-top: 4px;">The Acareca Team</span></p>
                        </td>
                    </tr>

                    <tr>
                        <td style="background-color: #fafafa; padding: 20px 40px; border-top: 1px solid #f1f5f9; text-align: center;">
                            <p style="margin: 0; color: #94a3b8; font-size: 12px; line-height: 18px;">This is an automated transactional receipt. Please do not reply directly to this message.</p>
                        </td>
                    </tr>

                </table>
            </td>
        </tr>
    </table>
</body>
</html>`
)

// GetTemplateContext decides whether to return database values or system defaults to the API frontend
func GetTemplateContext(dbSubject, dbBody string) (string, string, bool) {
	isCustom := true

	subject := dbSubject
	if strings.TrimSpace(subject) == "" {
		subject = DefaultInvoicePaidSubject
		isCustom = false
	}

	body := dbBody
	if strings.TrimSpace(body) == "" {
		body = DefaultInvoicePaidBody
		isCustom = false
	}

	return subject, body, isCustom
}

// RenderTemplateReplacements handles executing placeholder token swapping at execution runtime
func RenderTemplateReplacements(subjectTemplate, bodyTemplate, contactName, invoiceNumber string) (string, string) {
	replacer := strings.NewReplacer(
		"{contact_name}", contactName,
		"{invoice_number}", invoiceNumber,
	)
	return replacer.Replace(subjectTemplate), replacer.Replace(bodyTemplate)
}
