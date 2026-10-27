#include <SDL3/SDL.h>
#include <assert.h>
#include <stdio.h>

int main(int argc, char **argv)
{
    (void)argv;
    const bool filtered = argc == 1;
    assert(SDL_Init(SDL_INIT_EVENTS));
    SDL_Event event = {0};
    event.type = SDL_EVENT_KEY_DOWN;
    event.key.scancode = SDL_SCANCODE_E;
    event.key.down = true;
    assert(SDL_PushEvent(&event));
    event.key.repeat = true;
    for (int i = 0; i < 30; ++i) assert(SDL_PushEvent(&event) == !filtered);
    event.type = SDL_EVENT_KEY_UP;
    event.key.down = false;
    event.key.repeat = false;
    assert(SDL_PushEvent(&event));
    event.type = SDL_EVENT_TEXT_INPUT;
    event.text.text = "e";
    assert(SDL_PushEvent(&event));
    int down = 0, up = 0, text = 0;
    while (SDL_PollEvent(&event)) {
        down += event.type == SDL_EVENT_KEY_DOWN;
        up += event.type == SDL_EVENT_KEY_UP;
        text += event.type == SDL_EVENT_TEXT_INPUT;
    }
    assert(down == (filtered ? 1 : 31) && up == 1 && text == 1);
    SDL_Quit();
    puts(filtered ? "Held E: one press, no repeat presses, one release; text input preserved"
                  : "Unrelated process: normal keyboard repeat preserved");
}
