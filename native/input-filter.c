#ifndef _GNU_SOURCE
#define _GNU_SOURCE
#endif
#include <SDL3/SDL.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

static bool SDLCALL physical_keys_only(void *userdata, SDL_Event *event)
{
    (void)userdata;
    return event->type != SDL_EVENT_KEY_DOWN || !event->key.repeat;
}

__attribute__((constructor)) static void install_filter(void)
{
    // LD_PRELOAD is inherited by helper processes. Only filter rosemary itself.
    char executable[4096];
    ssize_t length = readlink("/proc/self/exe", executable, sizeof(executable) - 1);
    if (length < 0) return;
    executable[length] = '\0';
    const char *name = strrchr(executable, '/');
    if (name == NULL || strcmp(name + 1, "rosemary") != 0) return;
    SDL_SetEventFilter(physical_keys_only, NULL);
}
