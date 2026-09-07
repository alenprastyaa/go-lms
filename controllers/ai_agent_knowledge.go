package controllers

import "strings"

// Peta aplikasi yang diberikan kepada asisten agar dapat memandu pengguna,
// bukan hanya menjalankan tool. Isinya mengikuti struktur menu yang sebenarnya
// ada pada aplikasi; jangan menambah menu yang tidak ada di sidebar.

const agentKnowledgeSuperAdmin = `
Menu yang tersedia untuk super admin:
- Dashboard: ringkasan jumlah sekolah, pengguna, guru, dan siswa.
- Sekolah: menambah sekolah, mengubah datanya, serta membuat akun admin sekolah.
- Setting Modul: menyalakan atau mematikan modul untuk tiap sekolah.
- Kelola Paket: paket langganan yang ditawarkan kepada sekolah.
- CMS Landing: mengelola isi halaman publik dan blog.
- Billing: tagihan langganan sekolah.
- Setting Admin: pengaturan akun super admin.

Catatan: pengisian data akademik, kurikulum, siswa, dan guru dikerjakan oleh
admin masing-masing sekolah melalui akunnya sendiri, bukan lewat akun super admin.
`

const agentKnowledgeSchoolAdmin = `
Menu yang tersedia untuk admin sekolah:

Kesiswaan:
- Jurusan: daftar jurusan atau peminatan sekolah.
- Kelas: daftar kelas beserta tingkat dan wali kelasnya.
- Siswa: data induk peserta didik.

Akademik & Kurikulum, dikerjakan berurutan sesuai nomor pada menu:
- Tahun Ajaran: menetapkan tahun ajaran dan semester yang sedang aktif.
- 1. Mapel: mendaftarkan mata pelajaran. Hanya nama mapel yang wajib diisi.
- 2. Ruang & Lab: mendaftarkan ruang belajar dan laboratorium.
- 3. Beban Guru: menentukan guru pengampu tiap mapel beserta jumlah jamnya.
- 4. Distribusi Kelas: membagikan beban guru tersebut ke kelas-kelas.
- 5. Slot Jadwal: menyiapkan slot waktu pelajaran dalam sepekan.
- 6. Generate Jadwal: menyusun jadwal otomatis dari data langkah 1 sampai 5.

Menu lain:
- Ujian Resmi: pelaksanaan ujian sekolah.
- Absensi: rekap dan pengaturan presensi.
- Sarpras dan Koperasi: inventaris serta penjualan koperasi sekolah.
- Pesan, Pengumuman, Laporan WhatsApp: komunikasi dengan warga sekolah.
- Karyawan: akun guru dan tenaga kependidikan.
- Billing dan Setting: langganan serta pengaturan sekolah.
`

// agentWorkflowNotes memuat urutan pengerjaan yang sering ditanyakan.
const agentWorkflowNotes = `
Urutan pengerjaan yang penting:

Menyiapkan akademik dan kurikulum dari nol:
1. Pastikan Tahun Ajaran aktif sudah ditetapkan.
2. Isi Jurusan, lalu Kelas, lalu Siswa pada kelompok menu Kesiswaan.
3. Pastikan akun guru sudah ada pada menu Karyawan.
4. Kerjakan menu Akademik & Kurikulum berurutan dari nomor 1 sampai 6.
   Tiap langkah memakai hasil langkah sebelumnya, jadi urutannya tidak boleh dilompati.
   Generate Jadwal pada langkah 6 hanya berhasil bila langkah 1 sampai 5 sudah terisi.

Menyiapkan sekolah baru:
1. Super admin membuat sekolah pada menu Sekolah.
2. Super admin membuat akun admin sekolah.
3. Selanjutnya admin sekolah tersebut yang mengisi jurusan, kelas, siswa, guru,
   serta akademik dan kurikulum.
`

func agentKnowledgeForRole(role string) string {
	parts := []string{}
	switch role {
	case "SUPER_ADMIN":
		parts = append(parts, agentKnowledgeSuperAdmin, agentKnowledgeSchoolAdmin)
	default:
		parts = append(parts, agentKnowledgeSchoolAdmin)
	}
	parts = append(parts, agentWorkflowNotes)
	return strings.TrimSpace(strings.Join(parts, "\n"))
}
