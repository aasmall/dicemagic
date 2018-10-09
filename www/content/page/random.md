---
title: "How Random?"
date: 2018-05-27T09:59:03-07:00
draft: false
---
# This Random

Each dice roll uses consumes 8 bytes of cryptographically strong randomness from `/dev/urandom` or possibly `getrandom(2)`. There are [countless](https://security.stackexchange.com/questions/3936/is-a-rand-from-dev-urandom-secure-for-a-login-key) [internet](https://pthree.org/2014/07/21/the-linux-random-number-generator/) [debates](https://www.2uo.de/myths-about-urandom/) about various random number generators, but I've used this one plenty for very sensitive operations, and have yet to break it.

But when it comes to dice, we sometimes go a [little crazy](https://www.awesomedice.com/blog/353/d20-dice-randomness-test-chessex-vs-gamescience/). Trying to make sure they are fair. Dice Magic is no different. With each new build, a test is run to ensure sufficient randomness it run by performing a [Chi-squared test](https://en.wikipedia.org/wiki/Chi-squared_test) on a few hundred thousand dice rolls:

```cmd
PASS: TestRoll/8d4 (12.48s)
roll_test.go:137: Rolling 8d4 500000 times
roll_test.go:138: Bucket : Probability : Expected : Observed
roll_test.go:139: ------------------------------------------
roll_test.go:144:      8 :  0.0015259% :   7.6294 :        6
roll_test.go:144:      9 :   0.012207% :   61.035 :       62
roll_test.go:144:     10 :   0.054932% :   274.66 :      305
roll_test.go:144:     11 :    0.18311% :   915.53 :     1002
roll_test.go:144:     12 :    0.49133% :   2456.7 :     2482
roll_test.go:144:     13 :     1.1108% :   5554.2 :     5468
roll_test.go:144:     14 :      2.179% :    10895 :    10837
roll_test.go:144:     15 :      3.772% :    18860 :    18738
roll_test.go:144:     16 :     5.8334% :    29167 :    29104
roll_test.go:144:     17 :     8.1299% :    40649 :    40746
roll_test.go:144:     18 :     10.266% :    51331 :    51288
roll_test.go:144:     19 :     11.792% :    58960 :    59039
roll_test.go:144:     20 :     12.347% :    61737 :    61374
roll_test.go:144:     21 :     11.792% :    58960 :    59011
roll_test.go:144:     22 :     10.266% :    51331 :    51459
roll_test.go:144:     23 :     8.1299% :    40649 :    40896
roll_test.go:144:     24 :     5.8334% :    29167 :    29165
roll_test.go:144:     25 :      3.772% :    18860 :    18808
roll_test.go:144:     26 :      2.179% :    10895 :    10941
roll_test.go:144:     27 :     1.1108% :   5554.2 :     5503
roll_test.go:144:     28 :    0.49133% :   2456.7 :     2507
roll_test.go:144:     29 :    0.18311% :   915.53 :      921
roll_test.go:144:     30 :   0.054932% :   274.66 :      273
roll_test.go:144:     31 :   0.012207% :   61.035 :       57
roll_test.go:144:     32 :  0.0015259% :   7.6294 :        8
roll_test.go:149: chi2=21.24914366676761, df=24, p=0.6239874617025
```

We also run some negative tests to prove the effacacy of the test. We detect variances as small as 1%
```cmd
PASS: TestRoll/3d20_1%_bias (1.52s)
roll_test.go:137: Rolling 3d20 100000 times
roll_test.go:138: Bucket : Probability : Expected : Observed
roll_test.go:139: ------------------------------------------
roll_test.go:144:      3 :     0.0125% :     12.5 :       12
roll_test.go:144:      4 :     0.0375% :     37.5 :       37
roll_test.go:144:      5 :      0.075% :       75 :       63
roll_test.go:144:      6 :      0.125% :      125 :      127
roll_test.go:144:      7 :     0.1875% :    187.5 :      188
roll_test.go:144:      8 :     0.2625% :    262.5 :      276
roll_test.go:144:      9 :       0.35% :      350 :      349
roll_test.go:144:     10 :       0.45% :      450 :      465
roll_test.go:144:     11 :     0.5625% :    562.5 :      549
roll_test.go:144:     12 :     0.6875% :    687.5 :      673
roll_test.go:144:     13 :      0.825% :      825 :      856
roll_test.go:144:     14 :      0.975% :      975 :      984
roll_test.go:144:     15 :     1.1375% :   1137.5 :     1133
roll_test.go:144:     16 :     1.3125% :   1312.5 :     1303
roll_test.go:144:     17 :        1.5% :     1500 :     1455
roll_test.go:144:     18 :        1.7% :     1700 :     1753
roll_test.go:144:     19 :     1.9125% :   1912.5 :     1897
roll_test.go:144:     20 :     2.1375% :   2137.5 :     2172
roll_test.go:144:     21 :      2.375% :     2375 :     2478
roll_test.go:144:     22 :      2.625% :     2625 :     2548
roll_test.go:144:     23 :       2.85% :     2850 :     2816
roll_test.go:144:     24 :       3.05% :     3050 :     2958
roll_test.go:144:     25 :      3.225% :     3225 :     3224
roll_test.go:144:     26 :      3.375% :     3375 :     3445
roll_test.go:144:     27 :        3.5% :     3500 :     3458
roll_test.go:144:     28 :        3.6% :     3600 :     3531
roll_test.go:144:     29 :      3.675% :     3675 :     3558
roll_test.go:144:     30 :      3.725% :     3725 :     3632
roll_test.go:144:     31 :       3.75% :     3750 :     4698
roll_test.go:144:     32 :       3.75% :     3750 :     3760
roll_test.go:144:     33 :      3.725% :     3725 :     3595
roll_test.go:144:     34 :      3.675% :     3675 :     3577
roll_test.go:144:     35 :        3.6% :     3600 :     3516
roll_test.go:144:     36 :        3.5% :     3500 :     3455
roll_test.go:144:     37 :      3.375% :     3375 :     3317
roll_test.go:144:     38 :      3.225% :     3225 :     3099
roll_test.go:144:     39 :       3.05% :     3050 :     3049
roll_test.go:144:     40 :       2.85% :     2850 :     2807
roll_test.go:144:     41 :      2.625% :     2625 :     2676
roll_test.go:144:     42 :      2.375% :     2375 :     2357
roll_test.go:144:     43 :     2.1375% :   2137.5 :     2150
roll_test.go:144:     44 :     1.9125% :   1912.5 :     1900
roll_test.go:144:     45 :        1.7% :     1700 :     1645
roll_test.go:144:     46 :        1.5% :     1500 :     1540
roll_test.go:144:     47 :     1.3125% :   1312.5 :     1327
roll_test.go:144:     48 :     1.1375% :   1137.5 :     1155
roll_test.go:144:     49 :      0.975% :      975 :      942
roll_test.go:144:     50 :      0.825% :      825 :      807
roll_test.go:144:     51 :     0.6875% :    687.5 :      675
roll_test.go:144:     52 :     0.5625% :    562.5 :      525
roll_test.go:144:     53 :       0.45% :      450 :      468
roll_test.go:144:     54 :       0.35% :      350 :      344
roll_test.go:144:     55 :     0.2625% :    262.5 :      263
roll_test.go:144:     56 :     0.1875% :    187.5 :      177
roll_test.go:144:     57 :      0.125% :      125 :      120
roll_test.go:144:     58 :      0.075% :       75 :       63
roll_test.go:144:     59 :     0.0375% :     37.5 :       39
roll_test.go:144:     60 :     0.0125% :     12.5 :       14
roll_test.go:149: chi2=296.5628402834637, df=57, p=0
roll_test.go:151: Biased to 31 1000 times
```

If any of the tests fail, the build is blocked until I can figure out why.