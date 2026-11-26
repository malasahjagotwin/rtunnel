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
    |                              |---- tunggu koneksi client <-------|  client dial
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
bins/             hasil build lokal (x86-64)
```

## 1. Jalankan agent di Pterodactyl

1. Unggah binary agent ke server panel, atau biarkan `start.sh` yang mengunduh.
   Cara paling cepat: unggah `bins/agent-linux-amd64` lewat file manager panel,
   ganti nama jadi `agent.bin`, lalu set **Startup Command** ke
   `chmod +x ./agent.bin && ./agent.bin`.
2. Kalau mau pakai `start.sh` (mengunduh otomatis), set **Startup Command** ke
   `bash start.sh` (atau `sh start.sh`).
3. Pastikan alokasi `14776` ada di server kamu. Wings mengirim `SERVER_PORT`,
   dan agent memakai nilai itu. Kalau kosong, agent pakai `14776`.
4. Jalankan. Log menampilkan `agent listening on ...`.

Token bersifat opsional: set variabel `TUNNEL_TOKEN` di menu **Startup** kalau
mau mengunci jalur tunnel (lihat bagian Keamanan). Tanpa itu, agent menerima
client pertama yang mengirim `RT1`.

## 2. Hubungkan CT103 ke agent

Jalankan di terminal CT103 (butuh `curl`):

```sh
curl -fsSL -o /tmp/rtclient https://github.com/malasahjagotwin/rtunnel/releases/latest/download/client-linux-amd64
chmod +x /tmp/rtclient
/tmp/rtclient -server 78.154.103.31:14776
```

Atau salin langsung file `bins/client-linux-amd64` ke CT103, rename jadi
`/tmp/rtclient`, lalu jalankan baris terakhir di atas.

Client akan men-dial agent dan menampilkan `tunnel up to ... exposing
127.0.0.1:22`. Biarkan berjalan. Kalau koneksi putus, client reconnect otomatis
dengan backoff.

Opsi:

- `-target 127.0.0.1:22` — alamat lokal yang diekspos (default sshd).
- `-server 78.154.103.31:14776` — alamat agent.
- `-token <TOKEN>` — harus sama dengan agent, kalau token dipakai.

Supaya tetap jalan setelah terminal ditutup:

```sh
nohup /tmp/rtclient -server 78.154.103.31:14776 >/tmp/rtclient.log 2>&1 &
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
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o bins/agent-linux-amd64 ./agent
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o bins/client-linux-amd64 ./client
```

Binary rilis dibuat otomatis saat push tag:

```sh
git tag v1.0.1
git push origin v1.0.1
```

GitHub Actions akan build `agent-linux-amd64`, `client-linux-amd64`, dan versi
`arm64`, lalu mengunggahnya ke Releases. `start.sh` dan perintah di CT103
mengambil dari rilis `latest`.

## Keamanan — wajib dibaca

- Port `14776` terbuka ke seluruh internet dan meneruskan ke sshd CT103. Siapa
  pun bisa mencoba login. **Jangan** biarkan autentikasi password aktif dengan
  kredensial lemah; pakai SSH key dan matikan `PasswordAuthentication`.
- Tanpa `TUNNEL_TOKEN`, siapa pun yang mengirim baris `RT1` ke port itu bisa
  mengklaim jalur tunnel dan menerima koneksi SSH user. Mereka bisa menyalurkan
  sesi SSH ke mesin lain atau mencegatnya. Pakai token kalau port ini publik.
- Menjalankan tunnel TCP umum dari game panel Pterodactyl biasanya melanggar
  ToS penyedia hosting. Akun panel bisa di-suspend. Periksa ToS-nya.
- Alat ini secara fungsi adalah backdoor: siapa pun yang memegang jalur ini
  punya jalan ke CT103. Pastikan pemilik CT103 sadar dan setuju.

## Catatan

- Hanya satu client tunnel aktif pada satu waktu. Client baru akan menggantikan
  yang lama.
- Kalau client belum tersambung, koneksi user di-drop (`no tunnel client` di log
  agent).
