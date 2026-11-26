#include <stdlib.h>
#include <math.h>
#include <gtk/gtk.h>
#ifdef GDK_WINDOWING_X11
#include <gdk/gdkx.h>
#endif
#ifdef HAVE_LAYER_SHELL
#include <gtk-layer-shell.h>
#endif
#include <libayatana-appindicator/app-indicator.h>
#include "ui.h"
#include "_cgo_export.h"

// The picker is one GtkDrawingArea painted with Cairo + Pango; the only real widget on it is a
// chromeless GtkEntry over the search box, kept for IME, selection and clipboard. All picker
// state lives here so paint() also runs headless (kp_render) with no GTK widgets at all.

static GtkWidget *win, *canvas, *entry, *hexentry;
static GdkVisual *rgba_visual; // the RGBA visual, so children can opt into alpha too
static AppIndicator *ind;
static GtkCssProvider *css;
static guint debounce, hide_timer;
static int layer, pointer_in; // layer: the window is a screen-sized layer-shell surface
static gint64 shown_at;       // monotonic time of the last show, for the first-open focus race
static int had_focus;         // did the surface hold keyboard focus since that show

typedef struct { char *face, *tags; int fav; } Tile;
typedef struct { char *label; int on; double x, w; } Chip; // x, w: content coords from the last paint
static GArray *tiles, *chips;
static char *count_text, *render_query;
static int sel, hover = -1, chip_hover = -1, grid_mode;
static double scroll_y, chip_scroll, chips_w;
static int cw = 1100, ch = 700; // panel size, the render size until the window is allocated
static double ox, oy;           // panel origin inside the window; event coords are shifted by it
static double pal[5][3];
enum { ACC, DIM, BG, TXT, SEC };
static PangoFontDescription *ui_font, *face_font;

typedef struct { double x, y, w, h; } R;
typedef struct { R header, search, chips, grid; int cols, cellW, cellH; } L;

// panel is the one place that maps a window size to the drawn panel: the whole window when
// floating, a centred min(1100, w-80) x min(700, h-80) box when it is a screen-sized layer surface.
static R panel(int w, int h) {
	if (!layer)
		return (R){0, 0, w, h};
	int pw = MAX(1, MIN(1100, w - 80)), ph = MAX(1, MIN(700, h - 80));
	return (R){(w - pw) / 2, (h - ph) / 2, pw, ph};
}

static L layout(int w, int h) {
	const double m = 12, gap = 8;
	L l;
	l.header = (R){m, m, w - 2 * m, 22};
	l.search = (R){m, l.header.y + l.header.h + gap, w - 2 * m, 32};
	l.chips = (R){m, l.search.y + l.search.h + gap, w - 2 * m, 22};
	l.grid = (R){m, l.chips.y + l.chips.h + gap, w - 2 * m, MAX(1, h - m - (l.chips.y + l.chips.h + gap))};
	// same maths as windows/picker.go layout()
	l.cols = MAX(3, (int)l.grid.w / 140);
	l.cellW = MAX(1, (int)l.grid.w / l.cols);
	l.cellH = MAX(1, (int)lround(l.cellW * 0.7));
	return l;
}

static int in(R r, double x, double y) { return x >= r.x && x < r.x + r.w && y >= r.y && y < r.y + r.h; }
static double content_h(L l) { return (double)((tiles->len + l.cols - 1) / l.cols) * l.cellH; }

static void ensure_state(void) {
	if (!tiles) {
		tiles = g_array_new(FALSE, TRUE, sizeof(Tile));
		chips = g_array_new(FALSE, TRUE, sizeof(Chip));
	}
}

static void redraw(void) {
	if (canvas)
		gtk_widget_queue_draw(canvas);
}

// clamp keeps both scroll offsets in range; it never loads more tiles.
static void clamp(L l) {
	scroll_y = MAX(0, MIN(scroll_y, content_h(l) - l.grid.h));
	chip_scroll = MAX(0, MIN(chip_scroll, chips_w - l.chips.w));
}

// settle appends the next page when the view nears the end of what is loaded, then clamps.
static void settle(void) {
	L l = layout(cw, ch);
	if (scroll_y + l.grid.h >= content_h(l) - 2 * l.cellH)
		goMore();
	clamp(l);
}

/* ---- painting ---- */

static void fonts_init(void) {
	if (ui_font)
		return;
	static const char *want[] = {"ProggyClean Nerd Font Propo", "Noto Sans Mono", NULL};
	const char *pick = "monospace";
	PangoFontFamily **fams;
	int n;
	pango_font_map_list_families(pango_cairo_font_map_get_default(), &fams, &n);
	for (int w = 0; want[w] && !g_strcmp0(pick, "monospace"); w++)
		for (int i = 0; i < n; i++)
			if (!g_ascii_strcasecmp(pango_font_family_get_name(fams[i]), want[w])) {
				pick = want[w];
				break;
			}
	g_free(fams);
	ui_font = pango_font_description_from_string(pick);
	face_font = pango_font_description_from_string("Noto Sans Mono CJK JP");
}

static const char *kp_ui_family(void) {
	fonts_init();
	return pango_font_description_get_family(ui_font);
}

static void col(cairo_t *cr, int k, double a) { cairo_set_source_rgba(cr, pal[k][0], pal[k][1], pal[k][2], a); }
static void white(cairo_t *cr, int a) { cairo_set_source_rgba(cr, 1, 1, 1, a / 255.0); }

static void rrect(cairo_t *cr, R r, double rad) {
	cairo_new_sub_path(cr);
	cairo_arc(cr, r.x + r.w - rad, r.y + rad, rad, -G_PI / 2, 0);
	cairo_arc(cr, r.x + r.w - rad, r.y + r.h - rad, rad, 0, G_PI / 2);
	cairo_arc(cr, r.x + rad, r.y + r.h - rad, rad, G_PI / 2, G_PI);
	cairo_arc(cr, r.x + rad, r.y + rad, rad, G_PI, 3 * G_PI / 2);
	cairo_close_path(cr);
}

// line is the path for a crisp 1 px stroke along the inside of r.
static void line(cairo_t *cr, R r, double rad) { rrect(cr, (R){r.x + .5, r.y + .5, r.w - 1, r.h - 1}, rad); }

// measure loads s at px into pl unconstrained and returns its logical width.
static int measure(PangoLayout *pl, PangoFontDescription *fd, double px, const char *s) {
	pango_font_description_set_absolute_size(fd, px * PANGO_SCALE);
	pango_layout_set_font_description(pl, fd);
	pango_layout_set_width(pl, -1);
	pango_layout_set_ellipsize(pl, PANGO_ELLIPSIZE_NONE);
	pango_layout_set_text(pl, s, -1);
	PangoRectangle lg;
	pango_layout_get_pixel_extents(pl, NULL, &lg);
	return lg.width;
}

// put draws what measure loaded, aligned in r, vertically centred, ellipsized to r's width.
static void put(cairo_t *cr, PangoLayout *pl, R r, PangoAlignment a) {
	pango_layout_set_width(pl, MAX(1, (int)r.w) * PANGO_SCALE);
	pango_layout_set_alignment(pl, a);
	pango_layout_set_ellipsize(pl, PANGO_ELLIPSIZE_END);
	PangoRectangle lg;
	pango_layout_get_pixel_extents(pl, NULL, &lg);
	cairo_move_to(cr, r.x, r.y + (r.h - lg.height) / 2.0);
	pango_cairo_show_layout(cr, pl);
}

static void text(cairo_t *cr, PangoLayout *pl, PangoFontDescription *fd, double px, const char *s, R r, PangoAlignment a) {
	measure(pl, fd, px, s);
	put(cr, pl, r, a);
}

/* ---- settings pane: drawn on the same canvas; only the hex GtkEntry is a real widget ---- */

static const char *presets[] = {"#ff6fae", "#3fa9f5", "#7bd88f", "#ffb454", "#b48ead", "#ff5555"};
#define NPRESET 6
static const char *opt_label[] = {"Paste after picking", "Keep picker on top", "Show tags under faces"};
static int set_open, set_opt[3], set_hover = -1;
static char *set_hex, *set_err; // set_hex stands in for the entry headless; set_err is Go's save error
enum { HIT_SWATCH = 0, HIT_OPT = 10, HIT_CANCEL = 20, HIT_SAVE = 21 };

typedef struct { R title, label, hex, err, sw[NPRESET], opt[3], cancel, save; } SL;

static SL slayout(int w, int h) {
	const double m = 12;
	SL s;
	s.title = (R){m, m, w - 2 * m, 22}; // same row as the picker header
	s.label = (R){m, s.title.y + s.title.h + 16, w - 2 * m, 18};
	s.hex = (R){m, s.label.y + s.label.h + 6, 160, 32};
	s.err = (R){m, s.hex.y + s.hex.h + 4, w - 2 * m, 16};
	for (int i = 0; i < NPRESET; i++)
		s.sw[i] = (R){m + i * 52, s.err.y + s.err.h + 8, 44, 22};
	for (int k = 0; k < 3; k++)
		s.opt[k] = (R){m, s.sw[0].y + s.sw[0].h + 20 + k * 30, 320, 24};
	s.save = (R){w - m - 90, h - m - 28, 90, 28};
	s.cancel = (R){s.save.x - 8 - 90, s.save.y, 90, 28};
	return s;
}

static int set_hit(double x, double y) {
	SL s = slayout(cw, ch);
	for (int i = 0; i < NPRESET; i++)
		if (in(s.sw[i], x, y))
			return HIT_SWATCH + i;
	for (int k = 0; k < 3; k++)
		if (in(s.opt[k], x, y))
			return HIT_OPT + k;
	return in(s.cancel, x, y) ? HIT_CANCEL : in(s.save, x, y) ? HIT_SAVE : -1;
}

static const char *hex_text(void) { return hexentry ? gtk_entry_get_text(GTK_ENTRY(hexentry)) : set_hex ? set_hex : ""; }

// valid_hex mirrors theme.go parseHex (optional '#', exactly six hex digits) for the live error line.
static int valid_hex(const char *s) {
	while (g_ascii_isspace(*s))
		s++;
	if (*s == '#')
		s++;
	int n = 0;
	for (; g_ascii_isxdigit(s[n]); n++)
		;
	for (int k = n; s[k]; k++)
		if (!g_ascii_isspace(s[k]))
			return 0;
	return n == 6;
}

static void rgb_hex(cairo_t *cr, const char *s) {
	unsigned v = (unsigned)strtoul(s + 1, NULL, 16);
	cairo_set_source_rgb(cr, (v >> 16 & 0xFF) / 255.0, (v >> 8 & 0xFF) / 255.0, (v & 0xFF) / 255.0);
}

static void button(cairo_t *cr, PangoLayout *pl, R r, const char *label, int hot) {
	line(cr, r, 3);
	white(cr, hot ? 0x18 : 0x0c);
	cairo_fill_preserve(cr);
	col(cr, hot ? ACC : DIM, 1);
	cairo_stroke(cr);
	col(cr, hot ? ACC : TXT, 1);
	text(cr, pl, ui_font, 14, label, r, PANGO_ALIGN_CENTER);
}

static void paint_settings(cairo_t *cr, PangoLayout *pl, int w, int h) {
	SL s = slayout(w, h);
	const char *q = hex_text();
	int ok = valid_hex(q);
	col(cr, TXT, 1);
	text(cr, pl, ui_font, 18, "Settings", s.title, PANGO_ALIGN_LEFT);
	text(cr, pl, ui_font, 14, "Accent colour", s.label, PANGO_ALIGN_LEFT);

	// hex box; the GtkEntry on top draws the live text, caret and selection
	line(cr, s.hex, 4);
	white(cr, 0x10);
	cairo_fill_preserve(cr);
	if (!ok)
		cairo_set_source_rgb(cr, 0xff / 255.0, 0x5f / 255.0, 0x6d / 255.0);
	else
		col(cr, !hexentry || gtk_widget_has_focus(hexentry) ? ACC : DIM, 1);
	cairo_stroke(cr);
	if (!hexentry) {
		col(cr, TXT, 1);
		text(cr, pl, ui_font, 14, q, (R){s.hex.x + 8, s.hex.y, s.hex.w - 16, s.hex.h}, PANGO_ALIGN_LEFT);
	}
	const char *err = set_err ? set_err : ok ? NULL : "accent must be #rrggbb";
	if (err) {
		cairo_set_source_rgb(cr, 0xff / 255.0, 0x5f / 255.0, 0x6d / 255.0);
		text(cr, pl, ui_font, 12, err, s.err, PANGO_ALIGN_LEFT);
	}

	char *key = g_strstrip(g_strdup(q));
	const char *k = *key == '#' ? key + 1 : key;
	for (int i = 0; i < NPRESET; i++) {
		R r = s.sw[i];
		if (!g_ascii_strcasecmp(k, presets[i] + 1)) { // selected: accent ring 2 px outside the swatch
			line(cr, (R){r.x - 3, r.y - 3, r.w + 6, r.h + 6}, 6);
			col(cr, ACC, 1);
			cairo_stroke(cr);
		}
		line(cr, r, 4);
		rgb_hex(cr, presets[i]);
		cairo_fill_preserve(cr);
		if (set_hover == HIT_SWATCH + i)
			col(cr, TXT, 1);
		else
			white(cr, 0x40);
		cairo_stroke(cr);
	}
	g_free(key);

	for (int o = 0; o < 3; o++) {
		R r = s.opt[o], box = {r.x, r.y + (r.h - 14) / 2, 14, 14};
		line(cr, box, 2);
		if (set_opt[o]) {
			col(cr, ACC, 1);
			cairo_fill_preserve(cr);
		}
		col(cr, set_opt[o] || set_hover == HIT_OPT + o ? ACC : DIM, 1);
		cairo_stroke(cr);
		if (set_opt[o]) {
			cairo_save(cr);
			cairo_set_line_width(cr, 2);
			cairo_move_to(cr, box.x + 3, box.y + 7);
			cairo_line_to(cr, box.x + 6, box.y + 10);
			cairo_line_to(cr, box.x + 11, box.y + 4);
			col(cr, BG, 1);
			cairo_stroke(cr);
			cairo_restore(cr);
		}
		col(cr, TXT, 1);
		text(cr, pl, ui_font, 14, opt_label[o], (R){box.x + 14 + 10, r.y, r.w - 24, r.h}, PANGO_ALIGN_LEFT);
	}

	button(cr, pl, s.cancel, "Cancel", set_hover == HIT_CANCEL);
	button(cr, pl, s.save, "Save", set_hover == HIT_SAVE);
}

// paint takes the window size; everything after the translate is in panel coordinates.
static void paint(cairo_t *cr, int ww, int wh) {
	fonts_init();
	ensure_state();
	R P = panel(ww, wh);
	int w = P.w, h = P.h;
	L l = layout(w, h);
	PangoLayout *pl = pango_cairo_create_layout(cr);
	cairo_set_line_width(cr, 1);

	// clear to transparent so the panel's rounded corners (and in layer mode the margins) show through
	cairo_save(cr);
	cairo_set_operator(cr, CAIRO_OPERATOR_SOURCE);
	cairo_set_source_rgba(cr, 0, 0, 0, 0);
	cairo_paint(cr);
	cairo_restore(cr);
	cairo_translate(cr, P.x, P.y);
	line(cr, (R){0, 0, w, h}, 6);
	// 0.93 alpha, matching the Quickshell overlay's #ee000000: the panel reads as glass over a
	// blurring compositor instead of an opaque slab, and text keeps its contrast.
	col(cr, BG, 0.93);
	cairo_fill_preserve(cr);
	col(cr, ACC, 1);
	cairo_stroke(cr);
	if (set_open) {
		paint_settings(cr, pl, w, h);
		g_object_unref(pl);
		return;
	}

	// header
	col(cr, TXT, 1);
	text(cr, pl, ui_font, 18, "Kaomoji Picker", l.header, PANGO_ALIGN_LEFT);
	col(cr, SEC, 1);
	text(cr, pl, ui_font, 14, count_text ? count_text : "", l.header, PANGO_ALIGN_RIGHT);

	// search box; the GtkEntry on top draws the typed text, caret and selection
	line(cr, l.search, 4);
	white(cr, 0x10);
	cairo_fill_preserve(cr);
	col(cr, !entry || gtk_widget_has_focus(entry) ? ACC : DIM, 1);
	cairo_stroke(cr);
	const char *q = entry ? gtk_entry_get_text(GTK_ENTRY(entry)) : render_query ? render_query : "";
	R inner = {l.search.x + 8, l.search.y, l.search.w - 16, l.search.h};
	if (!*q) {
		cairo_set_source_rgb(cr, 0x88 / 255.0, 0x88 / 255.0, 0x88 / 255.0);
		text(cr, pl, ui_font, 14, "search kaomoji / tag…  (arrows, Enter = copy, right-click = favourite, Esc = close)", inner, PANGO_ALIGN_LEFT);
	} else if (!entry) { // headless: stand in for the entry's own text
		col(cr, TXT, 1);
		text(cr, pl, ui_font, 14, q, inner, PANGO_ALIGN_LEFT);
	}

	// chip strip
	cairo_save(cr);
	cairo_rectangle(cr, l.chips.x, l.chips.y, l.chips.w, l.chips.h);
	cairo_clip(cr);
	double x = 0;
	for (guint i = 0; i < chips->len; i++) {
		Chip *c = &g_array_index(chips, Chip, i);
		c->x = x;
		c->w = measure(pl, ui_font, 12, c->label) + 18;
		x += c->w + 6;
		R r = {l.chips.x + c->x - chip_scroll, l.chips.y, c->w, l.chips.h};
		if (r.x >= l.chips.x + l.chips.w || r.x + r.w < l.chips.x)
			continue;
		line(cr, r, 3);
		if (c->on)
			col(cr, ACC, 0x33 / 255.0);
		else
			white(cr, (int)i == chip_hover ? 0x18 : 0x0c);
		cairo_fill_preserve(cr);
		col(cr, c->on ? ACC : DIM, 1);
		cairo_stroke(cr);
		col(cr, c->on ? ACC : SEC, 1);
		put(cr, pl, r, PANGO_ALIGN_CENTER);
	}
	chips_w = MAX(0, x - 6);
	cairo_restore(cr);

	// grid, only the visible rows
	cairo_save(cr);
	cairo_rectangle(cr, l.grid.x, l.grid.y, l.grid.w, l.grid.h);
	cairo_clip(cr);
	if (!tiles->len) {
		col(cr, SEC, 1);
		text(cr, pl, ui_font, 16, "No matches", l.grid, PANGO_ALIGN_CENTER);
	}
	for (guint i = (guint)(scroll_y / l.cellH) * l.cols; i < tiles->len; i++) {
		Tile *t = &g_array_index(tiles, Tile, i);
		int row = i / l.cols, c = i % l.cols;
		double y = l.grid.y + row * l.cellH - scroll_y;
		if (y >= l.grid.y + l.grid.h)
			break;
		R tile = {l.grid.x + c * l.cellW + 2, y + 2, l.cellW - 4, l.cellH - 4};
		int on = (int)i == sel;
		line(cr, tile, 4);
		if (on || (int)i == hover) {
			col(cr, ACC, (on ? 0x22 : 0x15) / 255.0);
			cairo_fill_preserve(cr);
		}
		col(cr, ACC, on ? 1 : 0x1a / 255.0);
		cairo_stroke(cr);

		R tag = {tile.x + 4, tile.y + tile.h - 4 - 14, tile.w - 8, 14};
		R face = {tile.x + 6, tile.y + 6, tile.w - 12, (*t->tags ? tag.y : tile.y + tile.h - 6) - (tile.y + 6)};
		// Text.HorizontalFit: shrink from 22 px to 11 px until it fits, then ellipsize
		double px = 22;
		while (px > 11 && measure(pl, face_font, px, t->face) > face.w)
			px--;
		col(cr, on ? ACC : TXT, 1);
		put(cr, pl, face, PANGO_ALIGN_CENTER);
		if (*t->tags) {
			col(cr, SEC, 1);
			text(cr, pl, ui_font, 11, t->tags, tag, PANGO_ALIGN_CENTER);
		}
		if (t->fav) {
			col(cr, ACC, 1);
			text(cr, pl, face_font, 12, "★", (R){tile.x + tile.w - 5 - 20, tile.y + 5, 20, 14}, PANGO_ALIGN_RIGHT);
		}
	}
	cairo_restore(cr);

	// scroll thumb in the right margin: no trough, no steppers
	double chh = content_h(l);
	if (chh > l.grid.h) {
		double th = MAX(24, l.grid.h * l.grid.h / chh);
		cairo_rectangle(cr, l.grid.x + l.grid.w + 4, l.grid.y + scroll_y * (l.grid.h - th) / (chh - l.grid.h), 3, th);
		col(cr, DIM, 1);
		cairo_fill(cr);
	}
	g_object_unref(pl);
}

int kp_render(const char *path, const char *query, int settings, int screen, int w, int h) {
	set_open = settings; // static state outlives one render (tests paint both views in one process)
	g_free(render_query);
	render_query = g_strdup(query);
	layer = screen; // headless: paint as the layer surface would, w x h being the output size
	R P = panel(w, h);
	cw = P.w, ch = P.h;
	ensure_state();
	clamp(layout(cw, ch));
	cairo_surface_t *s = cairo_image_surface_create(CAIRO_FORMAT_ARGB32, w, h);
	cairo_t *cr = cairo_create(s);
	paint(cr, w, h);
	cairo_destroy(cr);
	int st = cairo_surface_write_to_png(s, path);
	cairo_surface_destroy(s);
	return st;
}

void kp_set_palette(unsigned acc, unsigned dim, unsigned bg, unsigned txt, unsigned sec) {
	unsigned v[5] = {acc, dim, bg, txt, sec};
	for (int k = 0; k < 5; k++)
		for (int c = 0; c < 3; c++)
			pal[k][c] = (v[k] >> (16 - 8 * c) & 0xFF) / 255.0;
	redraw();
}

/* ---- GTK glue ---- */

// kp_diag reports what the display actually gives us, without showing a window: which GDK backend
// is in use, whether layer-shell is available, and whether an RGBA visual exists at all. Cheap to
// run inside a live session and it says exactly why a surface would come out opaque.
const char *kp_diag(void) {
	static char buf[1024];
	GdkDisplay *d = gdk_display_get_default();
	GdkScreen *sc = d ? gdk_screen_get_default() : NULL;
	GdkVisual *rv = sc ? gdk_screen_get_rgba_visual(sc) : NULL;
	int layer = 0, depth = -1, alpha = -1;
#ifdef HAVE_LAYER_SHELL
	layer = gtk_layer_is_supported();
#endif
	if (rv) {
		depth = gdk_visual_get_depth(rv);
		GdkVisualType t = gdk_visual_get_visual_type(rv);
		alpha = (t == GDK_VISUAL_TRUE_COLOR || t == GDK_VISUAL_DIRECT_COLOR) && depth == 32;
	}
	g_snprintf(buf, sizeof buf,
	           "display=%s type=%s\nvisual_rgba=%s depth=%d alpha_capable=%d\nlayer_shell_supported=%d layer_compiled=%d\n",
	           d ? gdk_display_get_name(d) : "(none)",
	           d ? G_OBJECT_TYPE_NAME(d) : "(none)", rv ? "yes" : "no", depth, alpha, layer,
#ifdef HAVE_LAYER_SHELL
	           1
#else
	           0
#endif
	);
	return buf;
}

// kp_diag_window builds the real window (never shown), realizes it, and reports the visuals and
// opaque region that actually reached the compositor. This is the fact the screen path depends on.
const char *kp_diag_window(void) {
	static char buf[1600];
	if (!win)
		kp_build();
	gtk_widget_realize(win);
	GdkWindow *gw = gtk_widget_get_window(win);
	GdkVisual *wv = gtk_widget_get_visual(win);
	GdkVisual *gvis = gw ? gdk_window_get_visual(gw) : NULL;
	g_snprintf(buf, sizeof buf,
	           "window_visual_depth=%d gdkwindow_visual_depth=%d\n"
	           "gdkwindow_rgba=%d\n"
	           "layer_mode=%d panel=%dx%d+%d+%d\n",
	           wv ? gdk_visual_get_depth(wv) : -1,
	           gvis ? gdk_visual_get_depth(gvis) : -1,
	           gvis && gdk_visual_get_depth(gvis) == 32,
	           layer, cw, ch, ox, oy);
	return buf;
}

int kp_init(void) {
	g_set_prgname("kaomoji-picker");
	if (!gtk_init_check(NULL, NULL))
		return 0;
	g_object_set(gtk_settings_get_default(), "gtk-application-prefer-dark-theme", TRUE, NULL);
	return 1;
}

void kp_main(void) { gtk_main(); }
void kp_quit(void) { gtk_main_quit(); }

static gboolean post_cb(gpointer d) {
	goCommand(GPOINTER_TO_INT(d));
	return G_SOURCE_REMOVE;
}

// kp_post is the only entry point safe to call from another thread.
void kp_post(int cmd) { g_idle_add(post_cb, GINT_TO_POINTER(cmd)); }

static gboolean search_fire(gpointer d) {
	debounce = 0;
	goSearch((char *)gtk_entry_get_text(GTK_ENTRY(entry)));
	return G_SOURCE_REMOVE;
}

static void on_changed(GtkEditable *e, gpointer d) {
	if (debounce)
		g_source_remove(debounce);
	debounce = g_timeout_add(60, search_fire, NULL);
	redraw(); // placeholder on/off
}

static void on_activate(GtkEntry *e, gpointer d) {
	if (debounce) {
		g_source_remove(debounce);
		search_fire(NULL);
	}
	if ((guint)sel < tiles->len)
		goPick(sel);
}

static void settings_close(void) {
	set_open = 0, set_hover = -1;
	g_clear_pointer(&set_err, g_free);
	if (hexentry) {
		gtk_widget_hide(hexentry);
		gtk_widget_set_can_focus(entry, TRUE);
		gtk_widget_show(entry);
		gtk_widget_grab_focus(entry);
	}
	redraw();
}

static void settings_save(void) {
	char *e = goSaveSettings((char *)hex_text(), set_opt[0], set_opt[1], set_opt[2]);
	if (!e) {
		settings_close();
		return;
	}
	g_free(set_err);
	set_err = g_strdup(e);
	free(e);
	redraw();
}

static void on_hex_changed(GtkEditable *e, gpointer d) {
	g_clear_pointer(&set_err, g_free);
	redraw(); // live error line and swatch ring
}

static void on_hex_activate(GtkEntry *e, gpointer d) { settings_save(); }

static void set_click(int hit) {
	if (hit >= HIT_SWATCH && hit < HIT_SWATCH + NPRESET) {
		if (hexentry)
			gtk_entry_set_text(GTK_ENTRY(hexentry), presets[hit - HIT_SWATCH]);
	} else if (hit >= HIT_OPT && hit < HIT_OPT + 3)
		set_opt[hit - HIT_OPT] ^= 1;
	else if (hit == HIT_CANCEL)
		settings_close();
	else if (hit == HIT_SAVE)
		settings_save();
	redraw();
}

static void move(int d) {
	if (!tiles->len)
		return;
	int t = sel + d;
	while (t >= (int)tiles->len) {
		guint before = tiles->len;
		goMore();
		if (tiles->len == before)
			break;
	}
	sel = CLAMP(t, 0, (int)tiles->len - 1);
	L l = layout(cw, ch);
	double top = (double)(sel / l.cols) * l.cellH;
	if (top < scroll_y)
		scroll_y = top;
	else if (top + l.cellH > scroll_y + l.grid.h)
		scroll_y = top + l.cellH - l.grid.h;
	settle();
	redraw();
}

// Keys behave as with the old flowbox: Down/PgDn from the search box enters the grid without
// moving, arrows and pages then move the selection, Up from the top row or typing goes back.
static gboolean on_key(GtkWidget *w, GdkEventKey *ev, gpointer d) {
	L l = layout(cw, ch);
	int page = MAX(1, (int)l.grid.h / l.cellH) * l.cols;
	if (set_open) { // Esc backs out to the picker; every other key goes to the hex entry, whatever has focus
		if (ev->keyval == GDK_KEY_Escape)
			settings_close();
		else if (hexentry && !gtk_widget_has_focus(hexentry)) {
			gtk_widget_grab_focus(hexentry);
			gtk_widget_event(hexentry, (GdkEvent *)ev);
		} else
			return FALSE;
		return TRUE;
	}
	switch (ev->keyval) {
	case GDK_KEY_Escape:
		kp_hide();
		return TRUE;
	case GDK_KEY_Down:
	case GDK_KEY_Page_Down:
		if (grid_mode)
			move(ev->keyval == GDK_KEY_Down ? l.cols : page);
		grid_mode = 1;
		return TRUE;
	case GDK_KEY_Up:
	case GDK_KEY_Page_Up:
		if (!grid_mode)
			return FALSE;
		if (ev->keyval == GDK_KEY_Up && sel < l.cols)
			grid_mode = 0;
		else
			move(ev->keyval == GDK_KEY_Up ? -l.cols : -page);
		return TRUE;
	case GDK_KEY_Left:
	case GDK_KEY_Right:
		if (!grid_mode)
			return FALSE;
		move(ev->keyval == GDK_KEY_Left ? -1 : 1);
		return TRUE;
	}
	if (gdk_keyval_to_unicode(ev->keyval) >= 0x21 && !(ev->state & (GDK_CONTROL_MASK | GDK_MOD1_MASK)))
		grid_mode = 0; // the entry has focus throughout, so the key itself just falls through to it
	return FALSE;
}

static int tile_at(double x, double y) {
	L l = layout(cw, ch);
	if (!in(l.grid, x, y))
		return -1;
	int c = (int)(x - l.grid.x) / l.cellW;
	int i = (int)((y - l.grid.y + scroll_y) / l.cellH) * l.cols + c;
	return c < l.cols && i < (int)tiles->len ? i : -1;
}

static int chip_at(double x, double y) {
	L l = layout(cw, ch);
	if (!in(l.chips, x, y))
		return -1;
	for (guint i = 0; i < chips->len; i++) {
		Chip *c = &g_array_index(chips, Chip, i);
		double cx = l.chips.x + c->x - chip_scroll;
		if (x >= cx && x < cx + c->w)
			return i;
	}
	return -1;
}

static void clear_opaque(GtkWidget *w) {
	GdkWindow *g = gtk_widget_get_window(w);
	if (g)
		gdk_window_set_opaque_region(g, NULL);
}

static gboolean on_map(GtkWidget *w, GdkEvent *e, gpointer d) {
	clear_opaque(w);
	return FALSE;
}

static gboolean on_draw(GtkWidget *w, cairo_t *cr, gpointer d) {
	clear_opaque(win); // the compositor reads this every frame, so keep it honest
	int ww = gtk_widget_get_allocated_width(w), wh = gtk_widget_get_allocated_height(w);
	R P = panel(ww, wh);
	cw = P.w, ch = P.h, ox = P.x, oy = P.y;
	paint(cr, ww, wh);
	return TRUE;
}

static void on_size(GtkWidget *w, GdkRectangle *a, gpointer d) {
	R P = panel(a->width, a->height);
	cw = P.w, ch = P.h, ox = P.x, oy = P.y;
	GdkWindow *gw = gtk_widget_get_window(win);
	if (layer && gw) { // only the panel takes input; clicks on the transparent margin reach the windows below
		cairo_region_t *rg = cairo_region_create_rectangle(&(cairo_rectangle_int_t){P.x, P.y, P.w, P.h});
		gdk_window_input_shape_combine_region(gw, rg, 0, 0);
		cairo_region_destroy(rg);
	}
	// GDK marks widgets opaque unless told otherwise; with an opaque region set, Hyprland (and other
	// compositors) composite the surface without alpha, so the panel's margins and rounded corners
	// show as solid black. Clearing the region is what actually makes the overlay see-through.
	if (gw)
		gdk_window_set_opaque_region(gw, NULL);
	settle();
}

static gboolean on_press(GtkWidget *w, GdkEventButton *ev, gpointer d) {
	if (ev->type != GDK_BUTTON_PRESS)
		return FALSE;
	double x = ev->x - ox, y = ev->y - oy;
	if (set_open) {
		if (ev->button == 1)
			set_click(set_hit(x, y));
		return TRUE;
	}
	int c = chip_at(x, y), i = tile_at(x, y);
	if (c >= 0 && ev->button == 1)
		goChip(c);
	else if (i >= 0 && ev->button == 1)
		goPick(i);
	else if (i >= 0 && ev->button == 3)
		goFav(i);
	return TRUE;
}

static gboolean on_motion(GtkWidget *w, GdkEventMotion *ev, gpointer d) {
	double x = ev->x - ox, y = ev->y - oy;
	if (set_open) {
		int s = set_hit(x, y);
		if (s != set_hover) {
			set_hover = s;
			redraw();
		}
		return TRUE;
	}
	int i = tile_at(x, y), c = chip_at(x, y);
	if (i != hover || c != chip_hover) {
		hover = i, chip_hover = c;
		if (i >= 0)
			sel = i;
		redraw();
	}
	return TRUE;
}

static gboolean on_leave(GtkWidget *w, GdkEventCrossing *ev, gpointer d) {
	hover = chip_hover = set_hover = -1;
	redraw();
	return FALSE;
}

// wheel over the chip strip scrolls it sideways, anywhere else scrolls the grid a row per notch
static gboolean on_scroll(GtkWidget *w, GdkEventScroll *ev, gpointer d) {
	if (set_open)
		return TRUE;
	double dx = 0, dy = 0;
	switch (ev->direction) {
	case GDK_SCROLL_UP: dy = -1; break;
	case GDK_SCROLL_DOWN: dy = 1; break;
	case GDK_SCROLL_LEFT: dx = -1; break;
	case GDK_SCROLL_RIGHT: dx = 1; break;
	case GDK_SCROLL_SMOOTH: gdk_event_get_scroll_deltas((GdkEvent *)ev, &dx, &dy); break;
	}
	L l = layout(cw, ch);
	if (in(l.chips, ev->x - ox, ev->y - oy))
		chip_scroll += (dx + dy) * 80;
	else
		scroll_y += dy * l.cellH;
	settle();
	redraw();
	return TRUE;
}

static gboolean place_entry(GtkOverlay *o, GtkWidget *w, GdkRectangle *a, gpointer d) {
	R P = panel(gtk_widget_get_allocated_width(GTK_WIDGET(o)), gtk_widget_get_allocated_height(GTK_WIDGET(o)));
	R box = w == hexentry ? slayout(P.w, P.h).hex : layout(P.w, P.h).search;
	int nh;
	gtk_widget_get_preferred_height(w, NULL, &nh);
	a->x = P.x + box.x + 8, a->width = box.w - 16, a->height = nh;
	a->y = P.y + box.y + (box.h - nh) / 2;
	return TRUE;
}

static gboolean hide_late(gpointer d) {
	hide_timer = 0;
	if (had_focus && !set_open && !pointer_in && !gtk_window_is_active(GTK_WINDOW(win)))
		kp_hide();
	return G_SOURCE_REMOVE;
}

// The very first show fires a focus-out before the surface has ever held keyboard focus (the
// compositor grants it a moment later), which used to close the panel the instant it appeared.
// So: only a window that actually HAD focus may auto-hide, and never within 400 ms of being shown.

static gboolean on_focus_in(GtkWidget *w, GdkEvent *ev, gpointer d) {
	had_focus = 1;
	g_clear_handle_id(&hide_timer, g_source_remove);
	return FALSE;
}

// Hyprland moves keyboard focus to whatever is under the pointer, so focus-out alone is not
// "the user went elsewhere": keep the pane open, ignore it while the pointer is on the window,
// and otherwise hide after a grace period that hide_late cancels if focus or the pointer returns.
static gboolean on_focus_out(GtkWidget *w, GdkEvent *ev, gpointer d) {
	if (!had_focus || set_open || pointer_in || hide_timer)
		return FALSE;
	if (g_get_monotonic_time() - shown_at < 400000)
		return FALSE;
	hide_timer = g_timeout_add(200, hide_late, NULL);
	return FALSE;
}

// pointer_in tracks the toplevel's own surface; INFERIOR crossings are moves onto child widgets.
static gboolean on_cross(GtkWidget *w, GdkEventCrossing *ev, gpointer d) {
	if (ev->detail != GDK_NOTIFY_INFERIOR)
		pointer_in = ev->type == GDK_ENTER_NOTIFY;
	return FALSE;
}

static gboolean on_delete(GtkWidget *w, GdkEvent *ev, gpointer d) {
	kp_hide();
	return TRUE;
}

void kp_build(void) {
	ensure_state();
	win = gtk_window_new(GTK_WINDOW_TOPLEVEL);
	gtk_widget_set_name(win, "kp-root");
	gtk_window_set_title(GTK_WINDOW(win), "Kaomoji Picker");
	gtk_window_set_decorated(GTK_WINDOW(win), FALSE);
	gtk_window_set_skip_taskbar_hint(GTK_WINDOW(win), TRUE);
	gtk_window_set_skip_pager_hint(GTK_WINDOW(win), TRUE);
#ifdef GDK_WINDOWING_X11
	if (GDK_IS_X11_DISPLAY(gdk_display_get_default())) // utility windows get no taskbar entry or WM chrome on X11
		gtk_window_set_type_hint(GTK_WINDOW(win), GDK_WINDOW_TYPE_HINT_UTILITY);
#endif
	gtk_window_set_position(GTK_WINDOW(win), GTK_WIN_POS_MOUSE);
	gtk_window_set_default_size(GTK_WINDOW(win), 1100, 700);
#ifdef HAVE_LAYER_SHELL
	if (gtk_layer_is_supported()) { // a screen-sized overlay surface; paint() centres the panel in it
		layer = 1;
		GtkWindow *gw = GTK_WINDOW(win);
		gtk_layer_init_for_window(gw);
		gtk_layer_set_layer(gw, GTK_LAYER_SHELL_LAYER_OVERLAY);
		for (int e = 0; e < GTK_LAYER_SHELL_EDGE_ENTRY_NUMBER; e++)
			gtk_layer_set_anchor(gw, e, TRUE);
		gtk_layer_set_exclusive_zone(gw, -1);
		gtk_layer_set_keyboard_mode(gw, GTK_LAYER_SHELL_KEYBOARD_MODE_ON_DEMAND);
		gtk_layer_set_namespace(gw, "kaomoji-picker");
	}
#endif
	GdkVisual *v = gdk_screen_get_rgba_visual(gtk_widget_get_screen(win));
	rgba_visual = v;
	if (v)
		gtk_widget_set_visual(win, v);
	gtk_widget_set_app_paintable(win, TRUE);
	gtk_widget_add_events(win, GDK_ENTER_NOTIFY_MASK | GDK_LEAVE_NOTIFY_MASK);
	g_signal_connect(win, "key-press-event", G_CALLBACK(on_key), NULL);
	g_signal_connect(win, "focus-in-event", G_CALLBACK(on_focus_in), NULL);
	g_signal_connect(win, "focus-out-event", G_CALLBACK(on_focus_out), NULL);
	g_signal_connect(win, "enter-notify-event", G_CALLBACK(on_cross), NULL);
	g_signal_connect(win, "leave-notify-event", G_CALLBACK(on_cross), NULL);
	g_signal_connect(win, "delete-event", G_CALLBACK(on_delete), NULL);
	g_signal_connect(win, "map-event", G_CALLBACK(on_map), NULL);

	GtkWidget *ov = gtk_overlay_new();
	gtk_widget_set_app_paintable(ov, TRUE);
	if (rgba_visual)
		gtk_widget_set_visual(ov, rgba_visual);
	gtk_container_add(GTK_CONTAINER(win), ov);
	canvas = gtk_drawing_area_new();
	gtk_widget_set_app_paintable(canvas, TRUE);
	if (rgba_visual)
		gtk_widget_set_visual(canvas, rgba_visual);
	gtk_widget_add_events(canvas, GDK_BUTTON_PRESS_MASK | GDK_POINTER_MOTION_MASK | GDK_LEAVE_NOTIFY_MASK |
					      GDK_SCROLL_MASK | GDK_SMOOTH_SCROLL_MASK);
	g_signal_connect(canvas, "draw", G_CALLBACK(on_draw), NULL);
	g_signal_connect(canvas, "size-allocate", G_CALLBACK(on_size), NULL);
	g_signal_connect(canvas, "button-press-event", G_CALLBACK(on_press), NULL);
	g_signal_connect(canvas, "motion-notify-event", G_CALLBACK(on_motion), NULL);
	g_signal_connect(canvas, "leave-notify-event", G_CALLBACK(on_leave), NULL);
	g_signal_connect(canvas, "scroll-event", G_CALLBACK(on_scroll), NULL);
	gtk_container_add(GTK_CONTAINER(ov), canvas);

	entry = gtk_entry_new();
	gtk_widget_set_name(entry, "kp-search");
	gtk_entry_set_has_frame(GTK_ENTRY(entry), FALSE);
	g_signal_connect(entry, "changed", G_CALLBACK(on_changed), NULL);
	g_signal_connect(entry, "activate", G_CALLBACK(on_activate), NULL);
	g_signal_connect_swapped(entry, "notify::has-focus", G_CALLBACK(gtk_widget_queue_draw), canvas);
	gtk_overlay_add_overlay(GTK_OVERLAY(ov), entry);

	hexentry = gtk_entry_new(); // the settings pane's accent field, shown only while the pane is open
	gtk_widget_set_name(hexentry, "kp-hex");
	gtk_entry_set_has_frame(GTK_ENTRY(hexentry), FALSE);
	gtk_entry_set_max_length(GTK_ENTRY(hexentry), 16);
	g_signal_connect(hexentry, "changed", G_CALLBACK(on_hex_changed), NULL);
	g_signal_connect(hexentry, "activate", G_CALLBACK(on_hex_activate), NULL);
	g_signal_connect_swapped(hexentry, "notify::has-focus", G_CALLBACK(gtk_widget_queue_draw), canvas);
	gtk_overlay_add_overlay(GTK_OVERLAY(ov), hexentry);
	g_signal_connect(ov, "get-child-position", G_CALLBACK(place_entry), NULL);
	gtk_widget_show_all(ov);
	gtk_widget_hide(hexentry);
}

void kp_set_css(const char *s) {
	if (!css) {
		css = gtk_css_provider_new();
		// USER + 1, not APPLICATION: ~/.config/gtk-3.0/gtk.css is loaded at USER priority and sets
		// `* { color; font-family }`, which would otherwise win over an APPLICATION provider.
		gtk_style_context_add_provider_for_screen(gdk_screen_get_default(), GTK_STYLE_PROVIDER(css),
							  GTK_STYLE_PROVIDER_PRIORITY_USER + 1);
	}
	char *full = g_strdup_printf("* { font-family: \"%s\"; }\n%s", kp_ui_family(), s);
	gtk_css_provider_load_from_data(css, full, -1, NULL);
	g_free(full);
}

void kp_show(int keep_above) {
	GdkDisplay *dpy = gdk_display_get_default();
	int x = 0, y = 0;
	gdk_device_get_position(gdk_seat_get_pointer(gdk_display_get_default_seat(dpy)), NULL, &x, &y);
	GdkMonitor *m = gdk_display_get_monitor_at_point(dpy, x, y);
	g_clear_handle_id(&hide_timer, g_source_remove);
	shown_at = g_get_monotonic_time();
	had_focus = 0;
	if (layer) {
#ifdef HAVE_LAYER_SHELL
		// shortcut: layer shell means Wayland, where GDK only knows the pointer relative to our own
		// surfaces, so m is a stale guess while hidden; NULL lets the compositor pick its focused
		// output (Hyprland: the one under the cursor). Upgrade if a global-pointer query appears.
		gtk_layer_set_monitor(GTK_WINDOW(win), NULL);
#endif
	} else {
		GdkRectangle wa = {0, 0, 1280, 800};
		if (m)
			gdk_monitor_get_workarea(m, &wa);
		int w = MIN(1100, wa.width - 80), h = MIN(700, wa.height - 80);
		gtk_window_resize(GTK_WINDOW(win), w, h);
		gtk_window_set_keep_above(GTK_WINDOW(win), keep_above);
		// GTK_WIN_POS_MOUSE is only applied on first map, so place it explicitly too (ignored on Wayland)
		gtk_window_move(GTK_WINDOW(win), CLAMP(x - w / 2, wa.x, wa.x + wa.width - w), CLAMP(y - h / 2, wa.y, wa.y + wa.height - h));
	}
	grid_mode = 0;
	gtk_widget_show(win);
	gtk_window_present(GTK_WINDOW(win));
	GtkWidget *f = set_open ? hexentry : entry;
	gtk_widget_grab_focus(f);
	gtk_editable_select_region(GTK_EDITABLE(f), 0, -1);
}

// hiding always lands back on the picker; an open settings pane is cancelled
void kp_hide(void) {
	g_clear_handle_id(&hide_timer, g_source_remove);
	if (set_open)
		settings_close();
	gtk_widget_hide(win);
}
int kp_visible(void) { return gtk_widget_get_visible(win); }
void kp_set_keep_above(int on) {
	if (!layer) // layer surfaces on the overlay layer are always above
		gtk_window_set_keep_above(GTK_WINDOW(win), on);
}

int kp_layer_compiled(void) {
#ifdef HAVE_LAYER_SHELL
	return 1;
#else
	return 0;
#endif
}

void kp_set_count(const char *t) {
	g_free(count_text);
	count_text = g_strdup(t);
	redraw();
}

void kp_clear_chips(void) {
	ensure_state();
	for (guint i = 0; i < chips->len; i++)
		g_free(g_array_index(chips, Chip, i).label);
	g_array_set_size(chips, 0);
	chip_hover = -1;
	redraw();
}

void kp_add_chip(const char *label, int i) {
	ensure_state();
	Chip c = {g_strdup(label), 0, 0, 0};
	g_array_append_val(chips, c); // i == chips->len - 1; Go adds them in order
	redraw();
}

void kp_chip_state(int i, const char *label, int on) {
	if (i < 0 || (guint)i >= chips->len)
		return;
	Chip *c = &g_array_index(chips, Chip, i);
	g_free(c->label);
	c->label = g_strdup(label);
	c->on = on;
	redraw();
}

void kp_reset_tiles(void) {
	ensure_state();
	for (guint i = 0; i < tiles->len; i++) {
		g_free(g_array_index(tiles, Tile, i).face);
		g_free(g_array_index(tiles, Tile, i).tags);
	}
	g_array_set_size(tiles, 0);
	scroll_y = 0, hover = -1;
	redraw();
}

void kp_add_tile(const char *face, const char *tags, int fav) {
	ensure_state();
	Tile t = {g_strdup(face), g_strdup(tags), fav};
	g_array_append_val(tiles, t);
	redraw();
}

void kp_select_first(void) {
	sel = 0;
	redraw();
}

double kp_get_scroll(void) { return scroll_y; }

void kp_set_scroll(double v) {
	scroll_y = v;
	clamp(layout(cw, ch));
	redraw();
}

static void on_item(GtkMenuItem *m, gpointer d) { goMenu(GPOINTER_TO_INT(d)); }

void kp_tray(const char *icon) {
	static const char *labels[] = {"Open picker", "Settings…", "Reload data", NULL, "Exit"};
	static const int ids[] = {1, 2, 3, 0, 4};
	GtkWidget *menu = gtk_menu_new(), *open = NULL;
	for (int i = 0; i < 5; i++) {
		GtkWidget *it = labels[i] ? gtk_menu_item_new_with_label(labels[i]) : gtk_separator_menu_item_new();
		if (labels[i])
			g_signal_connect(it, "activate", G_CALLBACK(on_item), GINT_TO_POINTER(ids[i]));
		if (ids[i] == 1)
			open = it;
		gtk_menu_shell_append(GTK_MENU_SHELL(menu), it);
	}
	gtk_widget_show_all(menu);
	ind = app_indicator_new("kaomoji-picker", icon, APP_INDICATOR_CATEGORY_APPLICATION_STATUS);
	app_indicator_set_title(ind, "Kaomoji Picker");
	app_indicator_set_menu(ind, GTK_MENU(menu));
	app_indicator_set_secondary_activate_target(ind, open); // middle click opens the picker
	app_indicator_set_status(ind, APP_INDICATOR_STATUS_ACTIVE);
}

void kp_set_icon(const char *icon) {
	if (ind)
		app_indicator_set_icon_full(ind, icon, "Kaomoji Picker");
}
void kp_copy(const char *t) { gtk_clipboard_set_text(gtk_clipboard_get(GDK_SELECTION_CLIPBOARD), t, -1); }

// kp_settings opens the pane on the canvas; headless (no window) it only sets the state kp_render paints.
void kp_settings(const char *hex, int paste, int top, int tags) {
	g_free(set_hex);
	set_hex = g_strdup(hex);
	set_opt[0] = paste, set_opt[1] = top, set_opt[2] = tags;
	g_clear_pointer(&set_err, g_free);
	set_open = 1, set_hover = -1;
	if (hexentry) {
		gtk_entry_set_text(GTK_ENTRY(hexentry), hex);
		gtk_widget_hide(entry);
		gtk_widget_set_can_focus(entry, FALSE); // the hidden search entry must not take the keys meant for the hex field
		gtk_widget_show(hexentry);
		gtk_widget_grab_focus(hexentry);
		gtk_editable_select_region(GTK_EDITABLE(hexentry), 0, -1);
	}
	redraw();
}
