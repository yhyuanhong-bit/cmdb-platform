from celery import Celery
from app.config import settings

celery_app = Celery("ingestion", broker=settings.celery_broker_url, backend=settings.redis_url)
# Multi-replica safety:
#   - task_acks_late + task_reject_on_worker_lost: if a worker is killed
#     mid-task (SIGKILL, crash, preemption), the broker re-queues the job
#     so another replica picks it up. Without this, SIGKILL would silently
#     lose the job because default behaviour acks on receipt.
#   - worker_prefetch_multiplier=1: a worker holds at most one unacked job
#     at a time, so a dying worker can't drag a batch of in-flight jobs
#     down with it.
celery_app.conf.update(task_serializer="json", accept_content=["json"], result_serializer="json",
                       timezone="UTC", enable_utc=True, task_track_started=True, task_acks_late=True,
                       task_reject_on_worker_lost=True,
                       worker_prefetch_multiplier=1)
