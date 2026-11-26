# rtunnel

Reverse TCP tunnel satu port. Dibuat buat kasus: sebuah container (di balik NAT)
mau diakses SSH dari luar, tapi cuma punya **satu** alamat publik — di sini
Pterodactyl di `78.154.103.31:14776`.

Si container men-dial keluar ke agent yang publik, jadi container itu sendiri
tidak butuh IP publik dan tidak ada port yang dibuka di sisi router. Agent
menerima koneksi datang dari dua jenis: si client yang men-dial masuk, dan
orang yang SSH masuk. Baris pertama koneksi yang membedakan keduanya.

## Cara kerja

```
SSH client (kamu)                  agent (Pterodactyl)            client (CT103)
    |                              :14776                             |
    |---- TCP connect ------------> terima                              |
    |                              |---- tunggu koneksi client <-------|  client dial + token
    |                              |---- buka stream smux ----------->|  client dial 127.0.0.1:22
    |<======= relay byte dua arah =|===== TCP ke sshd CT103 =========>|  sshd CT103
```

Handshake SSH terjadi langsung antara SSH client kamu dan sshd CT103. Agent dan
client hanya memindahkan byte, tidak pernah menyimpan atau membaca kredensial.
Artinya login SSH tetap memakai kredensial CT103 sendiri — termasuk SSH key,
yang langsung jalan tanpa konfigurasi tambahan.

## Isi repo

```
agent/main.go     binary yang jalan di Pterodactyl, memegang port publik
client/main.go    binary yang jalan di CT103, men-dial keluar ke agent
start.sh          skrip startup untuk container Pterodactyl
.github/workflows/release.yml   build + rilis binary otomatis saat push tag
```

## 1. Jalankan agent di Pterodactyl

1. Set variabel `TUNNEL_TOKEN` di menu **Startup** panel ke sebuah string acak
   yang panjang. Ini yang mencegah orang lain mengklaim jalur tunnel kamu.
   Buat dengan: `openssl rand -hex 32`
2. Set **Startup Command** ke: `bash start.sh`
   (kalau `bash` tidak ada di image, pakai `sh start.sh`)
3. Pastikan alokasi `14776` ada di server kamu. Wings mengirim `SERVER_PORT`,
   dan agent memakai nilai itu. Kalau kosong, agent pakai `14776`.
4. Start server. Log akan menampilkan `agent listening on ...`.

`start.sh` mengunduh binary agent dari GitHub Releases sesuai arsitektur
container, jadi kamu tidak perlu build atau upload binary secara manual.

## 2. Hubungkan CT103 ke agent

Jalankan di terminal CT103 (butuh `curl`):

```sh
ARCH=$(uname -m); case "$ARCH" in x86_64) A=amd64 ;; aarch64|arm64) A=arm64 ;; *) echo "arch $ARCH" ; exit 1 ;; esac
curl -fsSL -o /tmp/rtclient "https://github.com/malasahjagotwin/rtunnel/releases/latest/download/client-linux-$A"
chmod +x /tmp/rtclient
TUNNEL_TOKEN='GANTI_DENGAN_TOKEN_YANG_SAMA' /tmp/rtclient -server 78.154.103.31:14776
```

Client akan men-dial agent, dan menampilkan `tunnel up to ... exposing
127.0.0.1:22`. Biarkan berjalan. Kalau koneksi putus, client reconnect
otomatis dengan backoff.

Opsi:

- `-target 127.0.0.1:22` — alamat lokal yang diekspos (default sshd).
- `-server 78.154.103.31:14776` — alamat agent.

Supaya tetap jalan setelah terminal ditutup:

```sh
nohup env TUNNEL_TOKEN='TOKEN' /tmp/rtclient -server 78.154.103.31:14776 >/tmp/rtclient.log 2>&1 &
```

## 3. Login SSH

Setelah client tersambung:

```sh
ssh -p 14776 root@78.154.103.31
```

### Pakai SSH key (disarankan)

Tunnel tidak peduli metode autentikasi — key, password, atau lainnya semuanya
diteruskan mentah ke sshd CT103. Jadi cukup pasang key di CT103:

1. Buat key di komputer kamu (kalau belum ada):

   ```sh
   ssh-keygen -t ed25519 -C "rtunnel"
   ```

2. Salin **public key** (`~/.ssh/id_ed25519.pub`) ke CT103, ke
   `/root/.ssh/authorized_keys` (izin `700` untuk `.ssh`, `600` untuk
   `authorized_keys`). Bisa lewat console Proxmox, `pct exec`, atau sementara
   lewat password dulu.

3. Di CT103, edit `/etc/ssh/sshd_config`:

   ```
   PubkeyAuthentication yes
   PasswordAuthentication no
   PermitRootLogin prohibit-password
   ```

   Lalu restart sshd (`systemctl restart ssh` atau `service ssh restart`).

4. Login:

   ```sh
   ssh -p 14776 -i ~/.ssh/id_ed25519 root@78.154.103.31
   ```

## Build dari source

```sh
CGO_ENABLED=0 go build -o agent.bin ./agent
CGO_ENABLED=0 go build -o client.bin ./client
```

Binary rilis dibuat otomatis saat kamu push tag:

```sh
git tag v1.0.0
git push origin v1.0.0
```

GitHub Actions akan build `agent-linux-amd64`, `client-linux-amd64`, dan versi
`arm64`, lalu mengunggahnya ke Releases. `start.sh` dan perintah di CT103
mengambil dari rilis `latest`.

## Keamanan — wajib dibaca

- Port `14776` terbuka ke seluruh internet dan meneruskan ke sshd CT103. Siapa
  pun bisa mencoba login. **Jangan** biarkan autentikasi password aktif dengan
  kredensial lemah; pakai SSH key dan matikan `PasswordAuthentication`.
- `TUNNEL_TOKEN` adalah identitas client. Tanpa itu, orang lain bisa men-dial
  agent, mengklaim jalur tunnel, dan menerima koneksi SSH user — yaitu mengarah
  ke mesin mereka, bukan CT103. Jaga kerahasiaannya.
- Menjalankan tunnel TCP umum dari game panel Pterodactyl biasanya melanggar
  ToS penyedia hosting. Akun panel bisa di-suspend. Periksa ToS-nya.
- Alat ini secara fungsi adalah backdoor: siapa pun yang memegang jalur ini
  punya jalan ke CT103. Pastikan pemilik CT103 sadar dan setuju.

## Catatan

- Hanya satu client tunnel aktif pada satu waktu. Client baru akan menggantikan
  yang lama.
- Kalau client belum tersambung, koneksi user di-drop (`no tunnel client` di log
  agent).
