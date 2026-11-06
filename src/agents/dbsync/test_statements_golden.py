"""Every SQL text DBSync can send, exactly, per dialect: a dialect change shows here as a diff, not at a customer's server."""
from pathlib import Path
import sys
import unittest

sys.path.insert(0,str(Path(__file__).resolve().parent))
import statements

STAMP='2026-11-05 10:00:00.123456'


def builder(engine,schema=None,key='numeric',zoned=False):
    b=statements.Builder(engine,'orders','id','updated_at',['note','id'],schema)
    b.kinds={'key':key,'updated':'timestamp'};b.zoned=zoned
    return b


class MySQL(unittest.TestCase):
    def test_every_shape(self):
        b=builder('mysql','shop')
        self.assertEqual(b.page(None,900),
            'SELECT `id`, `updated_at`, `note` FROM `shop`.`orders` WHERE `updated_at` IS NOT NULL ORDER BY `updated_at`, `id` LIMIT 900')
        self.assertEqual(b.page((STAMP,'42'),900),
            "SELECT `id`, `updated_at`, `note` FROM `shop`.`orders` WHERE `updated_at` IS NOT NULL AND (`updated_at` > '2026-11-05 10:00:00.123456'"
            " OR (`updated_at` = '2026-11-05 10:00:00.123456' AND `id` > 42)) ORDER BY `updated_at`, `id` LIMIT 900")
        self.assertEqual(b.page(None,900,STAMP),
            "SELECT `id`, `updated_at`, `note` FROM `shop`.`orders` WHERE `updated_at` IS NOT NULL AND `updated_at` >= '2026-11-05 10:00:00.123456'"
            " ORDER BY `updated_at`, `id` LIMIT 900")
        self.assertEqual(b.row('42'),'SELECT `id`, `updated_at`, `note` FROM `shop`.`orders` WHERE `id` = 42')
        self.assertEqual(b.newest(),'SELECT `id`, `updated_at` FROM `shop`.`orders` WHERE `updated_at` IS NOT NULL ORDER BY `updated_at` DESC, `id` DESC LIMIT 1')
        self.assertEqual(b.cutoff(300),"SELECT DATE_FORMAT(NOW(6) - INTERVAL 300 SECOND, '%Y-%m-%d %H:%i:%s.%f')")
        self.assertEqual(b.cutoff(300,'ms'),'SELECT FLOOR(UNIX_TIMESTAMP(NOW(6)) * 1000) - 300000')
        self.assertEqual(b.cutoff(300,'s'),'SELECT FLOOR(UNIX_TIMESTAMP(NOW(6)) * 1) - 300')
        self.assertEqual(b.types(),"SELECT column_name, data_type FROM information_schema.columns WHERE table_schema = 'shop'"
                         " AND table_name = 'orders' AND column_name IN ('id', 'updated_at')")
        self.assertEqual(builder('mysql').types(),"SELECT column_name, data_type FROM information_schema.columns WHERE table_schema = DATABASE()"
                         " AND table_name = 'orders' AND column_name IN ('id', 'updated_at')")


class PostgreSQL(unittest.TestCase):
    def test_every_shape(self):
        b=builder('postgres',key='timestamp')
        self.assertEqual(b.page((STAMP,STAMP),9),
            'SELECT "id", "updated_at", "note" FROM "orders" WHERE "updated_at" IS NOT NULL AND ("updated_at" > \'2026-11-05 10:00:00.123456\''
            ' OR ("updated_at" = \'2026-11-05 10:00:00.123456\' AND "id" > \'2026-11-05 10:00:00.123456\')) ORDER BY "updated_at", "id" LIMIT 9')
        self.assertEqual(b.page(None,9,STAMP),
            'SELECT "id", "updated_at", "note" FROM "orders" WHERE "updated_at" IS NOT NULL AND "updated_at" >= \'2026-11-05 10:00:00.123456\''
            ' ORDER BY "updated_at", "id" LIMIT 9')
        self.assertEqual(b.row(STAMP),'SELECT "id", "updated_at", "note" FROM "orders" WHERE "id" = \'2026-11-05 10:00:00.123456\'')
        self.assertEqual(b.newest(),'SELECT "id", "updated_at" FROM "orders" WHERE "updated_at" IS NOT NULL ORDER BY "updated_at" DESC, "id" DESC LIMIT 1')
        self.assertEqual(b.cutoff(300),"SELECT to_char(LOCALTIMESTAMP - interval '300 seconds', 'YYYY-MM-DD HH24:MI:SS.US')")
        self.assertEqual(builder('postgres',zoned=True).cutoff(300),
                         "SELECT to_char(CURRENT_TIMESTAMP - interval '300 seconds', 'YYYY-MM-DD HH24:MI:SS.USTZH:TZM')")
        self.assertEqual(b.cutoff(60,'s'),'SELECT FLOOR(EXTRACT(EPOCH FROM CURRENT_TIMESTAMP) * 1) - 60')
        self.assertEqual(b.types(),"SELECT column_name, data_type FROM information_schema.columns WHERE table_schema = current_schema()"
                         " AND table_name = 'orders' AND column_name IN ('id', 'updated_at')")
        self.assertEqual(builder('postgres','Shop').page(None,1),
            'SELECT "id", "updated_at", "note" FROM "Shop"."orders" WHERE "updated_at" IS NOT NULL ORDER BY "updated_at", "id" LIMIT 1')

    def test_a_lookback_and_a_limit_out_of_range_are_refused(self):
        b=builder('postgres')
        for bad in (0,86401,'300',1.5):
            with self.assertRaises(statements.StatementError):b.cutoff(bad)
        for bad in (0,10001,'9'):
            with self.assertRaises(statements.StatementError):b.page(None,bad)


if __name__=='__main__':unittest.main()
