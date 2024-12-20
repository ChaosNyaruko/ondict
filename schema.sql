DROP TABLE IF EXISTS history;
CREATE TABLE IF NOT EXISTS history (
    word TEXT NOT NULL UNIQUE,
    `count` INTEGER NOT NULL DEFAULT 0,
    create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX i_count on history(count);
CREATE INDEX i_latest on history(update_time);

-- PRAGMA recursive_triggers=0;
-- PRAGMA index_list(history);
-- PRAGMA index_info(i_latest);
-- select * from sqlite_master;


-- CREATE TRIGGER [UpdateLastTime]
--     AFTER INSERT
--     ON history
--     FOR EACH ROW
-- BEGIN
--     --UPDATE history SET count=count+1 WHERE word=OLD.word;
--     UPDATE history SET update_time=CURRENT_TIMESTAMP WHERE NEW.word=word;
-- END;

INSERT INTO history (word, count) VALUES ("doctor", 1) ON CONFLICT(word) DO UPDATE SET count=count+1, update_time=CURRENT_TIMESTAMP;

DROP TABLE IF EXISTS vocab;
CREATE TABLE IF NOT EXISTS vocab(
    word TEXT NOT NULL,
    src TEXT NOT NULL DEFAULT "",
    def TEXT NOT NULL DEFAULT ""
);

CREATE UNIQUE INDEX i_word_src ON vocab(word, src);
