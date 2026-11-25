-- How often a maintenance task comes back, e.g. every 6 months. Marking the
-- entry done logs it and adds the next one this far after the day it was done.
ALTER TABLE maintenance ADD COLUMN repeat_every INTEGER;
ALTER TABLE maintenance ADD COLUMN repeat_unit TEXT CHECK (repeat_unit IN ('week', 'month'));
