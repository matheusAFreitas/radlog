package app

import (
	"testing"
	"time"
)

// O celular (modo celular, só RC102) manda os minutos para o PC pela nuvem.
func TestCloudMobileToPC(t *testing.T) {
	fakeCloud(t)
	a, _ := testApp(t)
	b, _ := testApp(t)
	b.mobile = true
	old := mobileHost
	mobileHost = "SM-F741B"
	t.Cleanup(func() { mobileHost = old })

	cloudLogin(t, a)
	code, _ := a.NewCloudCode()
	if _, err := a.CloudFinishSetup(code); err != nil {
		t.Fatal(err)
	}
	a.cloudSync()

	cloudLogin(t, b)
	if _, err := b.CloudOpen(code); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Truncate(time.Minute)
	for i := 1; i <= 20; i++ {
		b.rcWriteMinute(rcMinute{t: now.Add(-time.Duration(i) * time.Minute), cps: 160, usv: 3, n: 20})
	}
	b.cloudSync()
	if st := b.GetCloud(); st.Err != "" || st.LastSync == 0 {
		t.Fatalf("celular depois de subir: %+v", st)
	}
	a.cloudSync()
	devs := a.GetCloud().Devices
	if len(devs) != 2 {
		t.Fatalf("aparelhos no PC: %+v", devs)
	}
	if pts := a.rcRange(now.Add(-time.Hour), now); len(pts) != 20 {
		t.Fatalf("PC recebeu %d minutos do RC102 do celular", len(pts))
	}
}
