#ifndef GPUI_GO_H
#define GPUI_GO_H
#include <stdint.h>
#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif
int gpui_go_init(void);
int gpui_go_run_headless(void);
int gpui_go_run_native(void);
void gpui_go_stop(void);
int gpui_go_headless_click(uint64_t entity, uint32_t generation, uint64_t revision, uint64_t element);
int gpui_go_headless_text_change(uint64_t entity, uint32_t generation, uint64_t element, uint64_t epoch, uint64_t edit_revision, const uint8_t *ptr, size_t len);
#ifdef __cplusplus
}
#endif

#endif
