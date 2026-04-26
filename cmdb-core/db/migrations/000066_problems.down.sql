DROP TABLE IF EXISTS problem_comments;
DROP TABLE IF EXISTS incident_problem_links;

DROP TRIGGER IF EXISTS problems_set_updated_at ON problems;
DROP FUNCTION IF EXISTS trg_problems_set_updated_at();

DROP TABLE IF EXISTS problems;
