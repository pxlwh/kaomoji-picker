#include <stdlib.h>
#include <gtk/gtk.h>
#include <libayatana-appindicator/app-indicator.h>
#include "ui.h"
#include "_cgo_export.h"

static GtkWidget *win, *entry, *countl, *chipbox, *flow, *gridscroll;
static AppIndicator *ind;
static GtkCssProvider *css;
static guint debounce;

int kp_init(void) {
	g_set_prgname("kaomoji-picker");
	return gtk_init_check(NULL, NULL);
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
}

static GtkFlowBoxChild *selected_child(void) {
	GList *l = gtk_flow_box_get_selected_children(GTK_FLOW_BOX(flow));
	GtkFlowBoxChild *c = l ? l->data : gtk_flow_box_get_child_at_index(GTK_FLOW_BOX(flow), 0);
	g_list_free(l);
	return c;
}

static void on_activate(GtkEntry *e, gpointer d) {
	if (debounce) {
		g_source_remove(debounce);
		search_fire(NULL);
	}
	GtkFlowBoxChild *c = selected_child();
	if (c)
		goPick(gtk_flow_box_child_get_index(c));
}

static void on_child(GtkFlowBox *f, GtkFlowBoxChild *c, gpointer d) {
	goPick(gtk_flow_box_child_get_index(c));
}

static gboolean on_press(GtkWidget *w, GdkEventButton *ev, gpointer d) {
	if (ev->type != GDK_BUTTON_PRESS || ev->button != 3 || ev->window != gtk_widget_get_window(flow))
		return FALSE;
	GtkFlowBoxChild *c = gtk_flow_box_get_child_at_pos(GTK_FLOW_BOX(flow), ev->x, ev->y);
	if (c)
		goFav(gtk_flow_box_child_get_index(c));
	return TRUE;
}

static void on_vadj(GtkAdjustment *a, gpointer d) {
	if (gtk_adjustment_get_value(a) + gtk_adjustment_get_page_size(a) >= gtk_adjustment_get_upper(a) - 250)
		goMore();
}

static gboolean on_key(GtkWidget *w, GdkEventKey *ev, gpointer d) {
	if (ev->keyval == GDK_KEY_Escape) {
		kp_hide();
		return TRUE;
	}
	GtkWidget *focus = gtk_window_get_focus(GTK_WINDOW(win));
	if (focus == entry && (ev->keyval == GDK_KEY_Down || ev->keyval == GDK_KEY_Page_Down)) {
		GtkFlowBoxChild *c = selected_child();
		if (c)
			gtk_widget_grab_focus(GTK_WIDGET(c));
		return TRUE;
	}
	// typing while the grid has focus goes to the search box
	if (focus != entry && gdk_keyval_to_unicode(ev->keyval) >= 0x21 &&
	    !(ev->state & (GDK_CONTROL_MASK | GDK_MOD1_MASK))) {
		gtk_entry_grab_focus_without_selecting(GTK_ENTRY(entry));
		return gtk_widget_event(entry, (GdkEvent *)ev);
	}
	return FALSE;
}

static gboolean on_focus_out(GtkWidget *w, GdkEvent *ev, gpointer d) {
	kp_hide();
	return FALSE;
}

static gboolean on_delete(GtkWidget *w, GdkEvent *ev, gpointer d) {
	kp_hide();
	return TRUE;
}

static void add_class(GtkWidget *w, const char *c) { gtk_style_context_add_class(gtk_widget_get_style_context(w), c); }

void kp_build(void) {
	win = gtk_window_new(GTK_WINDOW_TOPLEVEL);
	gtk_widget_set_name(win, "kp-root");
	gtk_window_set_title(GTK_WINDOW(win), "Kaomoji Picker");
	gtk_window_set_decorated(GTK_WINDOW(win), FALSE);
	gtk_window_set_skip_taskbar_hint(GTK_WINDOW(win), TRUE);
	gtk_window_set_skip_pager_hint(GTK_WINDOW(win), TRUE);
	gtk_window_set_type_hint(GTK_WINDOW(win), GDK_WINDOW_TYPE_HINT_DIALOG);
	gtk_window_set_position(GTK_WINDOW(win), GTK_WIN_POS_MOUSE);
	gtk_window_set_default_size(GTK_WINDOW(win), 1100, 700);
	g_signal_connect(win, "key-press-event", G_CALLBACK(on_key), NULL);
	g_signal_connect(win, "focus-out-event", G_CALLBACK(on_focus_out), NULL);
	g_signal_connect(win, "delete-event", G_CALLBACK(on_delete), NULL);

	GtkWidget *panel = gtk_box_new(GTK_ORIENTATION_VERTICAL, 8);
	gtk_widget_set_name(panel, "kp-panel");
	gtk_container_set_border_width(GTK_CONTAINER(win), 6);
	gtk_container_add(GTK_CONTAINER(win), panel);

	GtkWidget *header = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
	GtkWidget *title = gtk_label_new("Kaomoji Picker");
	gtk_widget_set_name(title, "kp-title");
	gtk_widget_set_hexpand(title, TRUE);
	gtk_widget_set_halign(title, GTK_ALIGN_START);
	countl = gtk_label_new("");
	gtk_widget_set_name(countl, "kp-count");
	gtk_box_pack_start(GTK_BOX(header), title, TRUE, TRUE, 0);
	gtk_box_pack_end(GTK_BOX(header), countl, FALSE, FALSE, 0);
	gtk_box_pack_start(GTK_BOX(panel), header, FALSE, FALSE, 0);

	entry = gtk_entry_new();
	gtk_entry_set_placeholder_text(GTK_ENTRY(entry), "Search…");
	g_signal_connect(entry, "changed", G_CALLBACK(on_changed), NULL);
	g_signal_connect(entry, "activate", G_CALLBACK(on_activate), NULL);
	gtk_box_pack_start(GTK_BOX(panel), entry, FALSE, FALSE, 0);

	GtkWidget *chipscroll = gtk_scrolled_window_new(NULL, NULL);
	gtk_scrolled_window_set_policy(GTK_SCROLLED_WINDOW(chipscroll), GTK_POLICY_EXTERNAL, GTK_POLICY_NEVER);
	chipbox = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 6);
	gtk_container_add(GTK_CONTAINER(chipscroll), chipbox);
	gtk_box_pack_start(GTK_BOX(panel), chipscroll, FALSE, FALSE, 0);

	gridscroll = gtk_scrolled_window_new(NULL, NULL);
	gtk_scrolled_window_set_policy(GTK_SCROLLED_WINDOW(gridscroll), GTK_POLICY_NEVER, GTK_POLICY_AUTOMATIC);
	gtk_widget_set_vexpand(gridscroll, TRUE);
	flow = gtk_flow_box_new();
	gtk_flow_box_set_homogeneous(GTK_FLOW_BOX(flow), TRUE);
	gtk_flow_box_set_selection_mode(GTK_FLOW_BOX(flow), GTK_SELECTION_SINGLE);
	gtk_flow_box_set_min_children_per_line(GTK_FLOW_BOX(flow), 3);
	gtk_flow_box_set_max_children_per_line(GTK_FLOW_BOX(flow), 40);
	gtk_flow_box_set_row_spacing(GTK_FLOW_BOX(flow), 4);
	gtk_flow_box_set_column_spacing(GTK_FLOW_BOX(flow), 4);
	gtk_widget_set_valign(flow, GTK_ALIGN_START);
	g_signal_connect(flow, "child-activated", G_CALLBACK(on_child), NULL);
	g_signal_connect(flow, "button-press-event", G_CALLBACK(on_press), NULL);
	gtk_container_add(GTK_CONTAINER(gridscroll), flow);
	g_signal_connect(gtk_scrolled_window_get_vadjustment(GTK_SCROLLED_WINDOW(gridscroll)), "value-changed", G_CALLBACK(on_vadj), NULL);
	gtk_box_pack_start(GTK_BOX(panel), gridscroll, TRUE, TRUE, 0);
	gtk_widget_show_all(panel);
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

void kp_set_css(const char *s) {
	if (!css) {
		css = gtk_css_provider_new();
		gtk_style_context_add_provider_for_screen(gdk_screen_get_default(), GTK_STYLE_PROVIDER(css),
							  GTK_STYLE_PROVIDER_PRIORITY_APPLICATION);
	}
	gtk_css_provider_load_from_data(css, s, -1, NULL);
}

void kp_show(int keep_above) {
	GdkDisplay *dpy = gdk_display_get_default();
	int x = 0, y = 0;
	gdk_device_get_position(gdk_seat_get_pointer(gdk_display_get_default_seat(dpy)), NULL, &x, &y);
	GdkMonitor *m = gdk_display_get_monitor_at_point(dpy, x, y);
	GdkRectangle wa = {0, 0, 1280, 800};
	if (m)
		gdk_monitor_get_workarea(m, &wa);
	int w = MIN(1100, wa.width - 80), h = MIN(700, wa.height - 80);
	gtk_window_resize(GTK_WINDOW(win), w, h);
	gtk_window_set_keep_above(GTK_WINDOW(win), keep_above);
	// GTK_WIN_POS_MOUSE is only applied on first map, so place it explicitly too (ignored on Wayland)
	gtk_window_move(GTK_WINDOW(win), CLAMP(x - w / 2, wa.x, wa.x + wa.width - w), CLAMP(y - h / 2, wa.y, wa.y + wa.height - h));
	gtk_widget_show(win);
	gtk_window_present(GTK_WINDOW(win));
	gtk_widget_grab_focus(entry);
	gtk_editable_select_region(GTK_EDITABLE(entry), 0, -1);
}

void kp_hide(void) { gtk_widget_hide(win); }
int kp_visible(void) { return gtk_widget_get_visible(win); }
void kp_set_keep_above(int on) { gtk_window_set_keep_above(GTK_WINDOW(win), on); }
void kp_set_count(const char *t) { gtk_label_set_text(GTK_LABEL(countl), t); }

static void destroy_children(GtkWidget *c) {
	GList *ch = gtk_container_get_children(GTK_CONTAINER(c));
	for (GList *l = ch; l; l = l->next)
		gtk_widget_destroy(l->data);
	g_list_free(ch);
}

static void on_chip(GtkButton *b, gpointer d) { goChip(GPOINTER_TO_INT(d)); }

void kp_clear_chips(void) { destroy_children(chipbox); }

void kp_add_chip(const char *label, int i) {
	GtkWidget *b = gtk_button_new_with_label(label);
	add_class(b, "kp-chip");
	gtk_widget_set_can_focus(b, FALSE);
	g_signal_connect(b, "clicked", G_CALLBACK(on_chip), GINT_TO_POINTER(i));
	gtk_widget_show(b);
	gtk_box_pack_start(GTK_BOX(chipbox), b, FALSE, FALSE, 0);
}

void kp_chip_state(int i, const char *label, int on) {
	GList *ch = gtk_container_get_children(GTK_CONTAINER(chipbox));
	GtkWidget *b = g_list_nth_data(ch, i);
	g_list_free(ch);
	if (!b)
		return;
	gtk_button_set_label(GTK_BUTTON(b), label);
	GtkStyleContext *sc = gtk_widget_get_style_context(b);
	if (on)
		gtk_style_context_add_class(sc, "kp-on");
	else
		gtk_style_context_remove_class(sc, "kp-on");
}

void kp_reset_tiles(void) {
	kp_set_scroll(0); // before clearing, so the stale upper bound cannot trigger goMore
	destroy_children(flow);
}

void kp_add_tile(const char *face, const char *tags, int fav) {
	GtkWidget *ov = gtk_overlay_new();
	GtkWidget *box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 2);
	gtk_widget_set_valign(box, GTK_ALIGN_CENTER);
	GtkWidget *f = gtk_label_new(face);
	add_class(f, "kp-face");
	gtk_label_set_ellipsize(GTK_LABEL(f), PANGO_ELLIPSIZE_END);
	gtk_label_set_max_width_chars(GTK_LABEL(f), 10); // homogeneous cells take the widest natural size
	gtk_box_pack_start(GTK_BOX(box), f, FALSE, FALSE, 0);
	if (*tags) {
		GtkWidget *t = gtk_label_new(tags);
		add_class(t, "kp-tags");
		gtk_label_set_ellipsize(GTK_LABEL(t), PANGO_ELLIPSIZE_END);
		gtk_label_set_max_width_chars(GTK_LABEL(t), 18);
		gtk_box_pack_start(GTK_BOX(box), t, FALSE, FALSE, 0);
	}
	gtk_container_add(GTK_CONTAINER(ov), box);
	if (fav) {
		GtkWidget *s = gtk_label_new("★");
		add_class(s, "kp-star");
		gtk_widget_set_halign(s, GTK_ALIGN_END);
		gtk_widget_set_valign(s, GTK_ALIGN_START);
		gtk_overlay_add_overlay(GTK_OVERLAY(ov), s);
	}
	gtk_widget_set_tooltip_text(ov, face);
	gtk_widget_set_size_request(ov, 140, 100);
	gtk_widget_show_all(ov);
	gtk_container_add(GTK_CONTAINER(flow), ov);
}

void kp_select_first(void) {
	GtkFlowBoxChild *c = gtk_flow_box_get_child_at_index(GTK_FLOW_BOX(flow), 0);
	if (c)
		gtk_flow_box_select_child(GTK_FLOW_BOX(flow), c);
}

double kp_get_scroll(void) { return gtk_adjustment_get_value(gtk_scrolled_window_get_vadjustment(GTK_SCROLLED_WINDOW(gridscroll))); }
void kp_set_scroll(double v) { gtk_adjustment_set_value(gtk_scrolled_window_get_vadjustment(GTK_SCROLLED_WINDOW(gridscroll)), v); }

void kp_copy(const char *t) { gtk_clipboard_set_text(gtk_clipboard_get(GDK_SELECTION_CLIPBOARD), t, -1); }

static GtkWidget *sdlg, *shex, *spaste, *stop, *stags, *serr;
static const char *presets[] = {"#ff6fae", "#3fa9f5", "#7bd88f", "#ffb454", "#b48ead", "#ff5555"};

static void on_swatch(GtkButton *b, gpointer d) { gtk_entry_set_text(GTK_ENTRY(shex), d); }

static void on_sresp(GtkDialog *d, int resp, gpointer u) {
	if (resp == GTK_RESPONSE_ACCEPT) {
		char *err = goSaveSettings((char *)gtk_entry_get_text(GTK_ENTRY(shex)),
					   gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(spaste)),
					   gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(stop)),
					   gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(stags)));
		if (err) {
			gtk_label_set_text(GTK_LABEL(serr), err);
			free(err);
			return;
		}
	}
	gtk_widget_destroy(sdlg);
	sdlg = NULL;
}

static GtkWidget *check(GtkWidget *box, const char *label, int on) {
	GtkWidget *c = gtk_check_button_new_with_label(label);
	gtk_toggle_button_set_active(GTK_TOGGLE_BUTTON(c), on);
	gtk_box_pack_start(GTK_BOX(box), c, FALSE, FALSE, 0);
	return c;
}

void kp_settings(const char *hex, int paste, int top, int tags) {
	if (sdlg) {
		gtk_window_present(GTK_WINDOW(sdlg));
		return;
	}
	sdlg = gtk_dialog_new_with_buttons("Kaomoji Picker Settings", NULL, 0, "_Cancel", GTK_RESPONSE_CANCEL,
					   "_Save", GTK_RESPONSE_ACCEPT, NULL);
	gtk_widget_set_name(sdlg, "kp-settings");
	gtk_dialog_set_default_response(GTK_DIALOG(sdlg), GTK_RESPONSE_ACCEPT);
	GtkWidget *box = gtk_dialog_get_content_area(GTK_DIALOG(sdlg));
	gtk_box_set_spacing(GTK_BOX(box), 8);
	gtk_container_set_border_width(GTK_CONTAINER(box), 12);
	GtkWidget *row = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 8);
	gtk_box_pack_start(GTK_BOX(row), gtk_label_new("Accent colour"), FALSE, FALSE, 0);
	shex = gtk_entry_new();
	gtk_entry_set_text(GTK_ENTRY(shex), hex);
	gtk_entry_set_width_chars(GTK_ENTRY(shex), 9);
	gtk_entry_set_activates_default(GTK_ENTRY(shex), TRUE);
	gtk_box_pack_start(GTK_BOX(row), shex, FALSE, FALSE, 0);
	gtk_box_pack_start(GTK_BOX(box), row, FALSE, FALSE, 0);
	GtkWidget *sw = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 4);
	for (int i = 0; i < 6; i++) {
		GtkWidget *b = gtk_button_new(), *l = gtk_label_new(NULL);
		char *mk = g_strdup_printf("<span background=\"%s\">      </span>", presets[i]);
		gtk_label_set_markup(GTK_LABEL(l), mk);
		g_free(mk);
		gtk_container_add(GTK_CONTAINER(b), l);
		gtk_widget_set_tooltip_text(b, presets[i]);
		g_signal_connect(b, "clicked", G_CALLBACK(on_swatch), (gpointer)presets[i]);
		gtk_box_pack_start(GTK_BOX(sw), b, FALSE, FALSE, 0);
	}
	gtk_box_pack_start(GTK_BOX(box), sw, FALSE, FALSE, 0);
	spaste = check(box, "Paste after picking", paste);
	stop = check(box, "Keep picker on top", top);
	stags = check(box, "Show tags under faces", tags);
	serr = gtk_label_new("");
	gtk_box_pack_start(GTK_BOX(box), serr, FALSE, FALSE, 0);
	g_signal_connect(sdlg, "response", G_CALLBACK(on_sresp), NULL);
	gtk_widget_show_all(sdlg);
}
