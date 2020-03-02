# create suitable mock data for displaying in scroll_mock

import sys
from random import randrange
import numpy as np

# used to give a range of different data points:
max_doors = 20
max_floors = 10

total_sensors = max_floors * max_doors

all_sensors = np.zeros(total_sensors, dtype=np.int32)

time_seconds = -1
time_minutes = 0
time_hours = 0

change_index = 0

door_sensor_changes = []


def random_door_change(num):
    global time_seconds
    global time_minutes
    global time_hours

    time_seconds += 1
    if time_seconds > 59:
        time_seconds = 0
        time_minutes += 1
        if time_minutes > 59:
            time_minutes = 0
            time_hours += 1

    if num == 0:
        return

    for n in range(num):
        global change_index
        change_index += 1
        door = randrange(max_doors)
        floor = randrange(max_floors)
        sensor = max_floors * floor + door
        # flip sensor state (open or close door)
        all_sensors[sensor] = 1 - all_sensors[sensor]

        mock_event = "{:02d}:".format(time_hours)
        mock_event += "{:02d}:".format(time_minutes)
        mock_event += "{:02d},".format(time_seconds)

        mock_event += "{:d},".format(change_index)

        mock_event += "{:d},".format(floor+1)
        mock_event += "{:d},".format(door+1)

        mock_event += "{:d}".format(all_sensors[sensor])

        door_sensor_changes.append(mock_event)


max_data = 100

changes_per_second = np.zeros(max_data, dtype=np.int32)

# adjust the following distribution as needed to create a simulation of your data
# ... or change the whole of this application to suit your use case.

changes_per_second[0] = 2104    # 0 changes in a second happened 2104 times
changes_per_second[1] = 1814
changes_per_second[2] = 1226
changes_per_second[3] = 864     # 3 changes in a second happened 864 times, and so on
changes_per_second[4] = 672
changes_per_second[5] = 558
changes_per_second[6] = 410
changes_per_second[7] = 374
changes_per_second[8] = 310
changes_per_second[9] = 270
changes_per_second[10] = 246
changes_per_second[11] = 218
changes_per_second[12] = 184
changes_per_second[13] = 172
changes_per_second[14] = 138
changes_per_second[15] = 102
changes_per_second[16] = 116
changes_per_second[17] = 104
changes_per_second[18] = 84
changes_per_second[19] = 88
changes_per_second[20] = 68
changes_per_second[21] = 62
changes_per_second[22] = 52
changes_per_second[23] = 52
changes_per_second[24] = 42
changes_per_second[25] = 36
changes_per_second[26] = 34
changes_per_second[27] = 30
changes_per_second[28] = 34
changes_per_second[29] = 24
changes_per_second[30] = 24
changes_per_second[31] = 22
changes_per_second[32] = 18
changes_per_second[33] = 16
changes_per_second[34] = 20
changes_per_second[35] = 16
changes_per_second[36] = 14
changes_per_second[37] = 18
changes_per_second[38] = 16
changes_per_second[39] = 12
changes_per_second[40] = 10
changes_per_second[41] = 6
changes_per_second[42] = 8
changes_per_second[43] = 6
changes_per_second[44] = 6
changes_per_second[45] = 8
changes_per_second[46] = 10
changes_per_second[47] = 4
changes_per_second[48] = 6
changes_per_second[49] = 4
changes_per_second[50] = 4
changes_per_second[51] = 4
changes_per_second[52] = 4
changes_per_second[53] = 2
changes_per_second[54] = 4
changes_per_second[55] = 2
changes_per_second[56] = 2
changes_per_second[57] = 6
changes_per_second[58] = 2
changes_per_second[59] = 2
changes_per_second[60] = 2
changes_per_second[61] = 2
changes_per_second[62] = 2
changes_per_second[63] = 2
changes_per_second[64] = 2
changes_per_second[65] = 2
changes_per_second[66] = 2
changes_per_second[67] = 2
changes_per_second[68] = 2
changes_per_second[69] = 2
changes_per_second[70] = 2
changes_per_second[71] = 2
changes_per_second[72] = 2
changes_per_second[73] = 2
changes_per_second[74] = 2
changes_per_second[75] = 2
changes_per_second[76] = 2
changes_per_second[77] = 2
changes_per_second[78] = 2
changes_per_second[79] = 2
changes_per_second[80] = 2
changes_per_second[81] = 2
changes_per_second[82] = 0
changes_per_second[83] = 0
changes_per_second[84] = 2
changes_per_second[85] = 0
changes_per_second[86] = 0
changes_per_second[87] = 0
changes_per_second[88] = 0
changes_per_second[89] = 2
changes_per_second[90] = 0
changes_per_second[91] = 2
changes_per_second[92] = 2
changes_per_second[93] = 2
changes_per_second[94] = 0
changes_per_second[95] = 0
changes_per_second[96] = 0
changes_per_second[97] = 2
changes_per_second[98] = 0
changes_per_second[99] = 2

total = 0
for i in range(max_data):
    total += changes_per_second[i]

mock_data = np.zeros(total, dtype=np.int32)

pos = 0

# spread the distribution out as an array of the number of changes for each second
for quantity in range(max_data):
    for n in range(changes_per_second[quantity]):
        mock_data[pos] = quantity
        pos += 1

print("total :", total)
print("pos :", pos)

# randomly cut out ONE item and expand that out into the stated number of events per second.
# this is done to maximize the sorts of data sets that this could represent.
# for example, a domain could contain 55 repeats of the same data values within 1 second
# this example data that is created does not do that but it could be changed to do so if yo need ...


attempts = 0
spins = 0
while pos > 0:
    i = randrange(total)
    if mock_data[i] > -1:
        # spit out random floor/door changes based on state for a time stamp
        random_door_change(mock_data[i])
        pos -= 1
        mock_data[i] = 0
        attempts = 0
    else:
        attempts += 1
        if attempts > 1000:
            attempts = 0
            # now search from random pos until a value found
            while 1:
                i += 1
                if i >= total:
                    i = 0
                    spins += 1
                    if spins > 5:
                        print("spinning")
                if mock_data[i] > -1:
                    # spit out random floor/door changes based on state for a time stamp
                    random_door_change(mock_data[i])
                    pos -= 1
                    mock_data[i] = 0
                    break

with open("mock_data.csv", 'w') as of:
    for line in door_sensor_changes:
        of.write(line+'\n')

sys.exit(0)
