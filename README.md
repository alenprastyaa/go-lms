# go-lms

## Email Provider

Email penawaran super admin dikirim melalui Brevo Transactional Email API.
Set environment variable berikut sebelum menjalankan backend:

```env
BREVO_API_KEY=isi_dengan_api_key_brevo
BREVO_SENDER_EMAIL=sender-terverifikasi@domain.com
BREVO_SENDER_NAME=Bitwize Digital Platform
BREVO_NO_REPLY_EMAIL=no-reply@domain.com
```

`BREVO_SENDER_EMAIL` harus sudah terdaftar dan terverifikasi sebagai sender/domain di Brevo.
`BREVO_NO_REPLY_EMAIL` dipakai sebagai Sender dan Reply-To untuk email penawaran agar balasan diarahkan ke alamat no-reply. Alamat/domain ini harus terverifikasi di Brevo.
