# termsaver

a terminal screensaver with multiple visualizations

## modes

| name              | description                                                                                                                   | 
|-------------------|-------------------------------------------------------------------------------------------------------------------------------|
| `matrix`          | classic falling characters effect with katakana, hiragana, and alphanumeric characters                                        | 
| `nyancat `        | animated rainbow-trailing cat flying through space                                                                            | 
| `snake`           | classic Nokia-style snake game (use arrow keys to play)                                                                       | 
| `missiledefender` | automatic tower defense game where towers defend against incoming missiles (towers and terrain randomize every 30-45 seconds) | 
| `spectrogragraph` | fake audio spectrograph with animated colored bars that continuously change                                                   |
| `snowflakes`      | falling snow that accumulates at the bottom and clears periodically                                                          |
| `waterripple`     | water drop rippling outwards from the center of the terminal                                                                 |
| `random`          | randomly selects one of the available modes                                                                                  | 

## usage

```bash
./termsaver -mode matrix   # Matrix rain effect
./termsaver -mode nyancat  # Flying rainbow cat
./termsaver -mode snake    # Snake game (automatic by default)
./termsaver -mode snake -interactive  # Snake game with manual control
./termsaver -mode missiledefender  # Tower defense game (fully automatic)
./termsaver -mode spectrograph  # Fake audio spectrograph with animated colored bars
./termsaver -mode snowflakes    # Falling snow that accumulates and clears periodically
./termsaver -mode waterripple   # Water drop rippling outwards from the center
./termsaver -mode random        # Randomly selects one of the available modes
```
