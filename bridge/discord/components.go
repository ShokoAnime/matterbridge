package bdiscord

import (
	"strings"

	"github.com/bwmarrin/discordgo"
)

// Discord's "components V2" messages (flag IS_COMPONENTS_V2) have no content or embeds: everything is in layout
// components (containers, text blocks, image galleries, buttons). Other platforms have none of that, so we flatten
// them into ordinary rich embeds, which the API bridge passes through and ShowEmbeds turns into text.

const maxEmbeds = 10
const maxDescription = 4096

// componentsToEmbeds turns components V2 into rich embeds: one per top-level container (or one for loose components),
// plus an image-only embed for each extra gallery image.
func componentsToEmbeds(components []discordgo.MessageComponent) []*discordgo.MessageEmbed {
	var out []*discordgo.MessageEmbed
	loose := &embedBuilder{}
	for _, c := range components {
		if ct, ok := c.(*discordgo.Container); ok {
			b := &embedBuilder{}
			if ct.AccentColor != nil {
				b.embed.Color = *ct.AccentColor
			}
			b.add(ct.Components)
			out = append(out, b.done()...)
			continue
		}
		loose.add([]discordgo.MessageComponent{c})
	}
	out = append(loose.done(), out...)
	if len(out) > maxEmbeds {
		out = out[:maxEmbeds]
	}
	return out
}

type embedBuilder struct {
	embed  discordgo.MessageEmbed
	parts  []string
	images []string
}

func (b *embedBuilder) add(components []discordgo.MessageComponent) {
	var links []string
	flushLinks := func() {
		if len(links) > 0 {
			b.parts = append(b.parts, strings.Join(links, " · "))
			links = nil
		}
	}
	for _, c := range components {
		switch v := c.(type) {
		case *discordgo.TextDisplay:
			flushLinks()
			if t := strings.TrimSpace(v.Content); t != "" {
				b.parts = append(b.parts, t)
			}
		case *discordgo.Section:
			flushLinks()
			b.add(v.Components)
			switch acc := v.Accessory.(type) {
			case *discordgo.Thumbnail:
				if acc.Media.URL != "" && b.embed.Thumbnail == nil {
					b.embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: acc.Media.URL}
				}
			case *discordgo.Button:
				if l := buttonLink(acc); l != "" {
					b.parts = append(b.parts, l)
				}
			}
		case *discordgo.MediaGallery:
			flushLinks()
			for _, it := range v.Items {
				if it.Media.URL != "" {
					b.images = append(b.images, it.Media.URL)
				}
			}
		case *discordgo.FileComponent:
			flushLinks()
			// attachment:// references point at the message's own files, which are bridged as attachments
			if u := v.File.URL; u != "" && !strings.HasPrefix(u, "attachment://") {
				b.parts = append(b.parts, u)
			}
		case *discordgo.Separator:
			flushLinks()
			if v.Divider == nil || *v.Divider {
				b.parts = append(b.parts, "───")
			}
		case *discordgo.ActionsRow:
			for _, x := range v.Components {
				if btn, ok := x.(*discordgo.Button); ok {
					if l := buttonLink(btn); l != "" {
						links = append(links, l)
					}
				}
			}
		case *discordgo.Container:
			flushLinks()
			b.add(v.Components)
		}
	}
	flushLinks()
}

// buttonLink is a markdown link for a link button; other buttons do something only Discord can, so they are dropped.
func buttonLink(btn *discordgo.Button) string {
	if btn == nil || btn.URL == "" {
		return ""
	}
	label := strings.TrimSpace(btn.Label)
	if label == "" {
		return btn.URL
	}
	return "[" + label + "](" + btn.URL + ")"
}

func (b *embedBuilder) done() []*discordgo.MessageEmbed {
	// separators at the edges separate nothing
	for len(b.parts) > 0 && b.parts[0] == "───" {
		b.parts = b.parts[1:]
	}
	for len(b.parts) > 0 && b.parts[len(b.parts)-1] == "───" {
		b.parts = b.parts[:len(b.parts)-1]
	}
	desc := strings.Join(b.parts, "\n")
	if r := []rune(desc); len(r) > maxDescription {
		desc = string(r[:maxDescription-1]) + "…"
	}
	b.embed.Type = discordgo.EmbedTypeRich
	b.embed.Description = desc
	if len(b.images) > 0 {
		b.embed.Image = &discordgo.MessageEmbedImage{URL: b.images[0]}
	}
	if desc == "" && b.embed.Image == nil && b.embed.Thumbnail == nil {
		return nil
	}
	out := []*discordgo.MessageEmbed{&b.embed}
	for _, u := range b.images[min(1, len(b.images)):] {
		out = append(out, &discordgo.MessageEmbed{Type: discordgo.EmbedTypeRich, Image: &discordgo.MessageEmbedImage{URL: u}})
	}
	return out
}
