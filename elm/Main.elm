module Main exposing (main)

--not super certain why elm-format puts the above line in (code compiles without it)

import Browser
import Entry
import HistoryView as Hv
import Html exposing (Html, div, input, label, option, select, text)
import Html.Attributes exposing (for, id, selected, style, type_, value)
import Html.Events exposing (keyCode, on, onClick, onInput, targetValue)
import Http
import Json.Decode as Json
import String
import Task


main : Program () Model Msg
main =
    Browser.element
        { init =
            List.head resourceList
                |> Maybe.withDefault ( "departures", "" )
                |> Tuple.first
                |> init
        , view = view
        , update = update
        , subscriptions = subscriptions
        }



-- INIT / MODEL


type alias Model =
    { resourceType : String
    , resourceId : String
    , entries : List Entry.Model
    , error : String
    , currentModel : Maybe Entry.Model
    , status : String
    , log : Hv.Log
    }


{-|

  - the () is the type for flags (we pass no flags to our program)

-}
init : String -> () -> ( Model, Cmd Msg )
init initType _ =
    ( { resourceType = initType
      , resourceId = ""
      , entries = []
      , error = ""
      , currentModel = Nothing
      , status = ""
      , log = Hv.empty
      }
    , Cmd.none
    )



-- UPDATE


type Msg
    = Entry String
    | GetAction
    | GetRecentAction
    | ChangeType String
    | ChangeVersion String
    | FetchOfData (Result Http.Error (List Entry.Model))
    | KeyPress Int
    | HistoryMsg Hv.Msg
    | EntryMsg Entry.Msg


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        Entry newId ->
            ( { model | resourceId = newId, status = "", error = "" }, Cmd.none )

        GetAction ->
            updateGetAction model

        GetRecentAction ->
            let
                newmodel =
                    { model | error = "", status = "Downloading, please wait ..." }
            in
            ( newmodel, getRecentData newmodel.resourceType )

        ChangeType newType ->
            let
                newmodel =
                    { model | resourceType = newType }
            in
            ( newmodel, Cmd.none )

        ChangeVersion strIndex ->
            let
                idx =
                    strIndex |> String.toInt |> Maybe.withDefault 0
            in
            ( { model | currentModel = lstGet idx model.entries }, Cmd.none )

        KeyPress key ->
            if key == 13 then
                {- 13 is enter / carriage return -}
                updateGetAction model

            else
                updateNoOp model

        HistoryMsg hmsg ->
            case hmsg of
                Hv.Clicked logEntry ->
                    { model
                        | resourceType = logEntry.resType
                        , resourceId = logEntry.resId
                    }
                        |> updateGetAction

        EntryMsg emsg ->
            updateNoOp model

        FetchOfData res ->
            case res of
                Ok entries ->
                    ( { model
                        | entries = entries
                        , currentModel = lstGet 0 entries
                        , error = ""
                        , status = (List.length entries |> String.fromInt) ++ " results found for " ++ model.resourceType ++ "/" ++ model.resourceId
                        , log =
                            case entries of
                                [] ->
                                    model.log

                                _ ->
                                    Hv.add
                                        { resId = model.resourceId
                                        , resType = model.resourceType
                                        , resTypeLabel = resourceLabel model.resourceType
                                        }
                                        model.log
                      }
                    , Cmd.none
                    )

                Err error ->
                    let
                        err =
                            errToString error
                    in
                    ( { model | error = err }, Cmd.none )


updateNoOp : Model -> ( Model, Cmd Msg )
updateNoOp model =
    ( model, Cmd.none )


updateGetAction : Model -> ( Model, Cmd Msg )
updateGetAction model =
    let
        newmodel =
            { model | error = "", status = "Downloading, please wait ..." }
    in
    ( newmodel, getData newmodel.resourceType newmodel.resourceId )



-- VIEW


{-| on change event so that FF works - added on trunk of elm but for now have this hack here
-}
onChange : (String -> Msg) -> Html.Attribute Msg
onChange tagger =
    on "change" (Json.map tagger targetValue)


onKeyPress : (Int -> Msg) -> Html.Attribute Msg
onKeyPress tagger =
    on "keypress" (Json.map tagger keyCode)


view : Model -> Html Msg
view model =
    let
        resview =
            case model.currentModel of
                Nothing ->
                    []

                Just entry ->
                    [ Html.map EntryMsg <| Entry.render entry ]

        status =
            if model.error == "" then
                text model.status

            else
                div [ style "color" "red" ] [ text model.error ]
    in
    div [ id "resapp" ]
        [ div [ id "resform" ]
            [ label [ for "restype" ] [ text "Resource: " ]
            , resourceList |> List.map (renderOption model.resourceType) |> select [ id "restype", onChange ChangeType ]
            , label [ for "resid" ] [ text "ID: " ]
            , input [ type_ "text", id "resid", value model.resourceId, onKeyPress KeyPress, onInput Entry ] []
            , input [ type_ "button", value "Get ID", onClick GetAction ] []
            , label [ for "recentbtn" ] [ text "Or Just" ]
            , input [ type_ "button", id "recentbtn", value "Get Recent", onClick GetRecentAction ] []
            , label [ for "reslst" ] [ text "Results: " ]
            , select [ id "reslst", onChange ChangeVersion ] <| renderResLst model.entries
            ]
        , div [ id "status" ] [ status ]
        , div [ id "resview" ] resview
        , Html.map HistoryMsg (Hv.view model.log)
        ]


renderResLst : List Entry.Model -> List (Html a)
renderResLst entries =
    entries |> toSelectTuples [] |> List.map (renderOption "")


renderOption : String -> ( String, String ) -> Html a
renderOption default ( val, label ) =
    let
        optval =
            if val == default then
                [ value val, selected True ]

            else
                [ value val ]
    in
    option optval [ text label ]



-- SUBSCRIPTIONS


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.none



-- LOGIC


type alias Resource =
    ( String, String )


resourceList : List Resource
resourceList =
    [ ( "departures", "Departures" )
    , ( "accommodations", "Accommodations" )
    , ( "accommodation_dossiers", "Accommodation Dossiers" )
    , ( "activities", "Activities" )
    , ( "activity_dossiers", "Activity Dossiers" )
    , ( "itineraries", "Itineraries" )
    , ( "packing_items", "Packing Items" )
    , ( "packing_lists", "Packing Lists" )
    , ( "place_dossiers", "Place Dossiers" )
    , ( "places", "Places" )
    , ( "promotions", "Promotions" )
    , ( "single_supplements", "Single Supplements" )
    , ( "tour_dossiers", "Tour Dossiers" )
    , ( "tours", "Tours" )
    , ( "transport_dossiers", "Transport Dossiers" )
    , ( "transports", "Transports" )
    ]


resourceLabel : String -> String
resourceLabel rType =
    resourceList
        |> List.filter (\x -> Tuple.first x == rType)
        |> List.head
        |> Maybe.withDefault ( "", "" )
        |> Tuple.second


getData : String -> String -> Cmd Msg
getData resType resId =
    let
        url =
            "/l/" ++ resType ++ "/" ++ resId
    in
    Http.get
        { expect = Http.expectJson FetchOfData decodeData
        , url = url
        }


getRecentData : String -> Cmd Msg
getRecentData resType =
    let
        url =
            "/lr/" ++ resType
    in
    Http.get
        { expect = Http.expectJson FetchOfData decodeData
        , url = url
        }


decodeData : Json.Decoder (List Entry.Model)
decodeData =
    Json.list Entry.decode


toSelectTuples : List ( String, String ) -> List Entry.Model -> List ( String, String )
toSelectTuples acc xs =
    case xs of
        [] ->
            List.reverse acc

        hd :: tl ->
            let
                newacc =
                    ( List.length acc |> String.fromInt, hd.fetchdate ) :: acc
            in
            toSelectTuples newacc tl


{-| helper func to retrieve item from list using index
-}
lstGet : Int -> List a -> Maybe a
lstGet index list =
    list |> List.drop index |> List.head


{-| helper to format http errors
-}
errToString : Http.Error -> String
errToString err =
    case err of
        Http.Timeout ->
            "Request Timeout"

        Http.NetworkError ->
            "Network Error"

        Http.BadStatus status ->
            "Unexpected HTTP status " ++ String.fromInt status

        Http.BadUrl url ->
            "Bad URL " ++ url

        Http.BadBody errmsg ->
            "Bad Body error: " ++ errmsg
