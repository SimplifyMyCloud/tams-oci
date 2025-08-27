# SSL Certificates

Place your SSL certificates in this directory:

- `tams.crt` - SSL certificate file
- `tams.key` - SSL private key file

For testing purposes, you can generate self-signed certificates:

```bash
openssl req -x509 -newkey rsa:4096 -keyout tams.key -out tams.crt -days 365 -nodes \
  -subj "/C=US/ST=State/L=City/O=Organization/CN=tams.example.com"
```

For production, use certificates from a trusted CA or Oracle's certificate management service.