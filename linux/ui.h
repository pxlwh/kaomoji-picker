// GTK3 + Ayatana AppIndicator glue. Implemented in ui.c, driven from ui.go.
int kp_init(void);
void kp_build(void);
void kp_main(void);
void kp_quit(void);
void kp_post(int cmd);

void kp_tray(const char *icon);
void kp_set_icon(const char *icon);
void kp_set_css(const char *css);
void kp_set_palette(unsigned accent, unsigned accent_dim, unsigned bg, unsigned text, unsigned secondary);
int kp_render(const char *path, const char *query, int settings, int screen, int w, int h); // headless, no GTK init; 0 = ok; screen: paint as a w x h layer surface

void kp_show(int keep_above);
void kp_hide(void);
int kp_visible(void);
void kp_set_keep_above(int on);
int kp_layer_compiled(void);

void kp_set_count(const char *text);
void kp_clear_chips(void);
void kp_add_chip(const char *label, int i);
void kp_chip_state(int i, const char *label, int on);

void kp_reset_tiles(void);
void kp_add_tile(const char *face, const char *tags, int fav);
void kp_select_first(void);
double kp_get_scroll(void);
void kp_set_scroll(double v);

void kp_copy(const char *text);
void kp_settings(const char *hex, int paste, int top, int tags); // safe headless
const char *kp_diag(void);
const char *kp_diag_window(void);
