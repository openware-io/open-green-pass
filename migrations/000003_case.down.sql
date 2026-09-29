-- GP1-02 回退：撤销用例版本化表（只用于本地重演/降级；已投产环境走新增版本不删表）
DROP TABLE IF EXISTS cas_version;
DROP TABLE IF EXISTS cas_case;
