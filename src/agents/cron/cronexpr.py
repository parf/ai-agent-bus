"""Five-field cron expressions: strict parsing, Vixie's day rule, local wall-clock slots."""
from datetime import datetime, timedelta, timezone
import re

# (name, low, high) per field; day of week 7 is folded to Sunday (0).
FIELDS = (('minute', 0, 59), ('hour', 0, 23), ('day of month', 1, 31), ('month', 1, 12), ('day of week', 0, 7))
# ASCII digits only: int() alone would also take '1_0', '+5', ' 5' and other scripts' digits.
ELEMENT = re.compile(r'(\*|([0-9]{1,2})(?:-([0-9]{1,2}))?)(?:/([0-9]{1,2}))?\Z')
DAYS = (31, 29, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31)  # February 29 can happen
HORIZON_DAYS = 8 * 366  # a February 29 schedule fires at least once in eight years


class CronError(ValueError):
    pass


class Spec:
    """Allowed values per field; dom_star/dow_star follow Vixie: the field starts with '*'."""

    def __init__(self, text, sets, dom_star, dow_star):
        self.text = text
        self.minute, self.hour, self.dom, self.month, self.dow = sets
        self.dom_star, self.dow_star = dom_star, dow_star

    def day(self, date):
        dom = date.day in self.dom
        dow = (date.weekday() + 1) % 7 in self.dow  # Python's Monday 0 is cron's 1
        # Vixie/cronie: either day field starting with '*' makes the match AND, else OR.
        return (dom and dow) if self.dom_star or self.dow_star else (dom or dow)

    def matches(self, wall):
        """Whether a naive local wall-clock minute is one of this schedule's slots."""
        return (wall.minute in self.minute and wall.hour in self.hour and wall.month in self.month
                and self.day(wall))


def field(text, position):
    name, low, high = FIELDS[position]
    values = set()
    for item in text.split(','):
        m = ELEMENT.match(item)
        if not m:
            raise CronError(f'field {position + 1} ({name}): {item!r} is not *, n, a-b or a step of them')
        star, first, last, step = m.group(1) == '*', m.group(2), m.group(3), m.group(4)
        a, b = (low, high) if star else (int(first), int(last if last is not None else first))
        if step is not None and not star and last is None:
            raise CronError(f'field {position + 1} ({name}): a step needs * or a range')
        if not low <= a <= high or not low <= b <= high:
            raise CronError(f'field {position + 1} ({name}): values are {low}..{high}')
        if a > b:
            raise CronError(f'field {position + 1} ({name}): range {a}-{b} runs backwards')
        n = int(step) if step is not None else 1
        if n == 0:
            raise CronError(f'field {position + 1} ({name}): step 0')
        values.update(range(a, b + 1, n))
    if position == 4:
        values = {0 if v == 7 else v for v in values}
    return values


def parse(text):
    if not isinstance(text, str) or not re.fullmatch(r'[^ ]+( [^ ]+){4}', text):
        raise CronError('a schedule is five fields separated by single spaces: minute hour day-of-month month day-of-week')
    parts = text.split(' ')
    spec = Spec(text, [field(p, i) for i, p in enumerate(parts)], parts[2].startswith('*'), parts[4].startswith('*'))
    if not any(spec.day(date) for date in sample_days(spec)):
        raise CronError('the schedule can never fire')
    return spec


def sample_days(spec):
    """One date per allowed (month, day), plus a leap February 29, across each weekday."""
    for month in sorted(spec.month):
        for day in range(1, DAYS[month - 1] + 1):
            for year in (2024, 2025, 2026, 2027, 2028, 2029, 2030):
                if month == 2 and day == 29 and year % 4:
                    continue
                yield datetime(year, month, day)


def wall(instant, zone):
    """The naive local wall-clock minute of an aware instant."""
    return instant.astimezone(zone).replace(second=0, microsecond=0, tzinfo=None)


def slot_key(minute):
    return minute.strftime('%Y-%m-%dT%H:%M')


def exists(minute, zone):
    """A wall-clock minute skipped by a spring-forward change does not exist."""
    aware = minute.replace(tzinfo=zone)
    return wall(aware.astimezone(timezone.utc), zone) == minute


def next_slot(spec, after, zone):
    """The first existing local slot strictly after the naive wall minute `after`, or None."""
    day = after.replace(hour=0, minute=0)
    for _ in range(HORIZON_DAYS):
        if day.month in spec.month and spec.day(day):
            for hour in sorted(spec.hour):
                for minute in sorted(spec.minute):
                    candidate = day.replace(hour=hour, minute=minute)
                    if candidate > after and exists(candidate, zone):
                        return candidate
        day += timedelta(days=1)
    return None
