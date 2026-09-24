package media

import "testing"

func TestYouTubeLinks(t *testing.T) {
	for _, link := range []string{"https://youtu.be/BaW_jenozKc", "https://www.youtube.com/watch?v=BaW_jenozKc&list=ignored", "https://m.youtube.com/shorts/BaW_jenozKc"} {
		got, err := canonical(link)
		if err != nil || got != "https://www.youtube.com/watch?v=BaW_jenozKc" {
			t.Fatal(got, err)
		}
	}
	for _, link := range []string{"https://youtube.com.evil.test/watch?v=BaW_jenozKc", "https://youtube.com/playlist?list=abc", "https://example.com/watch?v=BaW_jenozKc"} {
		if _, err := canonical(link); err == nil {
			t.Fatal(link)
		}
	}
}

func TestBoundedOutput(t *testing.T) {
	b := &boundedOutput{limit: 4}
	n, err := b.Write([]byte("123456"))
	if err != nil || n != 6 || string(b.data) != "1234" {
		t.Fatal(n, err, string(b.data))
	}
}
