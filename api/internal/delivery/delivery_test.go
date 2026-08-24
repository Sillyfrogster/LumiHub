package delivery

import (
	"testing"

	"github.com/Sillyfrogster/Illarin/api/internal/asset"
	"github.com/google/uuid"
)

func offered(ids ...string) []asset.DeliveryTarget {
	targets := make([]asset.DeliveryTarget, 0, len(ids))
	for _, id := range ids {
		targets = append(targets, asset.DeliveryTarget{Format: id, Label: id})
	}
	return targets
}

func TestTheInstancePicksTheFirstFormatItAcceptsThatIllarinOffers(t *testing.T) {
	chosen, label, found := chooseTarget(
		[]string{"card_v3", "card_v2"}, offered("card_v2", "card_v3"), false,
	)

	if !found || chosen != "card_v3" || label != "card_v3" {
		t.Fatalf("chooseTarget = %q, %q, %t, want card_v3", chosen, label, found)
	}
}

func TestAnAcceptedFormatIllarinDoesNotOfferSelectsNothing(t *testing.T) {
	chosen, _, found := chooseTarget(
		[]string{"invented_by_the_client", "card_v2"}, offered("card_v2"), false,
	)

	if !found || chosen != "card_v2" {
		t.Fatalf("chooseTarget = %q, %t, want card_v2", chosen, found)
	}
}

func TestAnAssetWithAnUploadedFileFallsBackToRaw(t *testing.T) {
	chosen, _, found := chooseTarget([]string{"card_v3"}, offered("card_v2"), true)

	if !found || chosen != asset.RawDownloadTarget {
		t.Fatalf("chooseTarget = %q, %t, want raw", chosen, found)
	}
}

func TestNothingIsChosenWhenNoFormatFitsAndThereIsNoUploadedFile(t *testing.T) {
	if _, _, found := chooseTarget([]string{"card_v3"}, offered("card_v2"), false); found {
		t.Fatal("chooseTarget found a target with nothing to fall back to")
	}
}

func TestASecondWaitSupersedesTheFirst(t *testing.T) {
	waiting := newHub(4)
	instanceID := uuid.New()

	first, admitted := waiting.hold(instanceID)
	if !admitted {
		t.Fatal("the first wait was not admitted")
	}
	second, admitted := waiting.hold(instanceID)
	if !admitted {
		t.Fatal("the second wait was not admitted")
	}

	select {
	case <-first.superseded:
	default:
		t.Fatal("the first wait was left hanging by the second")
	}
	select {
	case <-second.superseded:
		t.Fatal("the second wait was superseded by itself")
	default:
	}
}

func TestOneInstanceNeverHoldsTwoPlacesAtOnce(t *testing.T) {
	waiting := newHub(1)
	instanceID := uuid.New()

	if _, admitted := waiting.hold(instanceID); !admitted {
		t.Fatal("the first wait was not admitted")
	}
	if _, admitted := waiting.hold(instanceID); !admitted {
		t.Fatal("the same instance was refused its own place back")
	}
	if _, admitted := waiting.hold(uuid.New()); admitted {
		t.Fatal("a second instance was admitted past the limit")
	}
}

func TestReleasingLeavesAWaitThatAlreadySupersededItRegistered(t *testing.T) {
	waiting := newHub(2)
	instanceID := uuid.New()

	first, _ := waiting.hold(instanceID)
	second, _ := waiting.hold(instanceID)
	waiting.release(instanceID, first)
	waiting.signal(instanceID)

	select {
	case <-second.work:
	default:
		t.Fatal("the live wait lost its wake-up when a superseded one was released")
	}
}
